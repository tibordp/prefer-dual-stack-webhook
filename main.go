package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	klog "k8s.io/klog/v2"
)

type Result struct {
	Allowed  bool
	Msg      string
	PatchOps []PatchOperation
}

type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	From  string      `json:"from"`
	Value interface{} `json:"value,omitempty"`
}

func executeWebhook(r *admissionv1.AdmissionRequest) (*Result, error) {
	if r.Operation != admissionv1.Create {
		return &Result{Msg: fmt.Sprintf("Invalid operation: %s", r.Operation)}, nil
	}

	var operations []PatchOperation
	var service corev1.Service
	if err := json.Unmarshal(r.Object.Raw, &service); err != nil {
		return &Result{Msg: err.Error()}, nil
	}

	if service.Spec.IPFamilyPolicy == nil {
		operations = append(
			operations,
			PatchOperation{
				Op:    "add",
				Path:  "/spec/ipFamilyPolicy",
				Value: "PreferDualStack",
			},
		)
	}

	return &Result{
		Allowed:  true,
		PatchOps: operations,
	}, nil
}

func newAdmissionHandler() *admissionHandler {
	return &admissionHandler{
		decoder: serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer(),
	}
}

type admissionHandler struct {
	decoder runtime.Decoder
}

func (h *admissionHandler) Handle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, "invalid method only POST requests are allowed", http.StatusMethodNotAllowed)
			return
		}

		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			http.Error(w, "only content type 'application/json' is supported", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not read request body: %v", err), http.StatusBadRequest)
			return
		}

		var review admissionv1.AdmissionReview
		if _, _, err := h.decoder.Decode(body, nil, &review); err != nil {
			http.Error(w, fmt.Sprintf("could not deserialize request: %v", err), http.StatusBadRequest)
			return
		}

		if review.Request == nil {
			http.Error(w, "malformed admission review: request is nil", http.StatusBadRequest)
			return
		}

		result, err := executeWebhook(review.Request)
		if err != nil {
			klog.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		review.Response = &admissionv1.AdmissionResponse{
			UID:     review.Request.UID,
			Allowed: result.Allowed,
			Result:  &metav1.Status{Message: result.Msg},
		}

		if len(result.PatchOps) > 0 {
			patchBytes, err := json.Marshal(result.PatchOps)
			if err != nil {
				klog.Error(err)
				http.Error(w, fmt.Sprintf("could not marshal JSON patch: %v", err), http.StatusInternalServerError)
			}
			patchType := admissionv1.PatchTypeJSONPatch
			review.Response.PatchType = &patchType
			review.Response.Patch = patchBytes
		}

		res, err := json.Marshal(review)
		if err != nil {
			klog.Error(err)
			http.Error(w, fmt.Sprintf("could not marshal response: %v", err), http.StatusInternalServerError)
			return
		}

		klog.Infof("Webhook [%s - %s] - Allowed: %t", r.URL.Path, review.Request.Operation, result.Allowed)
		w.WriteHeader(http.StatusOK)
		w.Write(res)
	}
}

func healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

func NewServer(port string) *http.Server {
	ah := newAdmissionHandler()
	mux := http.NewServeMux()
	mux.Handle("/healthz", healthz())
	mux.Handle("/mutate/services", ah.Handle())

	return &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}
}

var (
	tlscert, tlskey, port string
)

func main() {
	flag.StringVar(&tlscert, "tlscert", "/etc/certs/tls.crt", "Path to the TLS certificate")
	flag.StringVar(&tlskey, "tlskey", "/etc/certs/tls.key", "Path to the TLS key")
	flag.StringVar(&port, "port", "8443", "The port to listen")
	flag.Parse()

	server := NewServer(port)
	go func() {
		if err := server.ListenAndServeTLS(tlscert, tlskey); err != nil {
			klog.Errorf("Failed to listen and serve: %v", err)
		}
	}()

	klog.Infof("Server running in port: %s", port)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	klog.Infof("Shutdown gracefully...")
	if err := server.Shutdown(context.Background()); err != nil {
		klog.Error(err)
	}
}
