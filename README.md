# PreferDualStack Admission Controller

Currently [Kubernetes services are created in single stack mode by default](https://kubernetes.io/docs/concepts/services-networking/dual-stack/#dual-stack-options-on-new-services), defaulting to primary address family of the cluster. 

This is a mutating webhook admission controller that adds `ipFamilyPolicy: PreferDualStack` to all newly created services if the field is not explicitely specified, making dual-stack the default.

## Installation

```
kubectl apply -f https://raw.githubusercontent.com/tibordp/prefer-dual-stack-webhook/master/deploy.yaml
```

## Demo

<p align="center">
<table>
<tr>
<th>Before</th>
<th>Afrer</th>
</tr>
<tr>
<td>

```bash
> kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: dummy
spec:
  selector:
    app: dummy
  ports:
    - port: 80
      targetPort: 80
EOF
service/dummy created

> kubectl describe svc dummy
Name:              dummy
Namespace:         default
Labels:            <none>
Annotations:       <none>
Selector:          app=dummy
Type:              ClusterIP
IP Family Policy:  SingleStack
IP Families:       IPv4
IP:                10.96.200.4
IPs:               10.96.200.4
Port:              <unset>  80/TCP
TargetPort:        80/TCP
Endpoints:         <none>
Session Affinity:  None
Events:            <none>
```
</td>
<td>

```bash
> kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: dummy
spec:
  selector:
    app: dummy
  ports:
    - port: 80
      targetPort: 80
EOF
service/dummy created

> kubectl describe svc dummy
Name:              dummy
Namespace:         default
Labels:            <none>
Annotations:       <none>
Selector:          app=dummy
Type:              ClusterIP
IP Family Policy:  PreferDualStack
IP Families:       IPv4,IPv6
IP:                10.96.19.47
IPs:               10.96.19.47,fd00:10:96::f31
Port:              <unset>  80/TCP
TargetPort:        80/TCP
Endpoints:         <none>
Session Affinity:  None
Events:            <none>
```
</td>
</tr>
</table>
</p>


## Acknowledgements

Admission controller webhook boilerplate adapted from [douglasmakey/admissioncontroller](https://github.com/douglasmakey/admissioncontroller) by Douglas Makey Mendez Molero.