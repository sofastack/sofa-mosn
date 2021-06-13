# MOSN 作为 istio-1.10.0 的数据面 sidecar

## 构建 MOSN 镜像

```bash
IMAGE_NAME=proxyv2 make sidecar
```

## 安装 istio

```bash
docker tag istio/pilot:1.10.0 192.168.8.88/mosnio/pilot:v0.23.0-3d007e03e-202106081720
```

```bash
/root/istio-1.10.0/bin/istioctl install --set profile=default --set meshConfig.enableTracing=false --set values.global.hub=192.168.8.88/mosnio --set values.global.tag=v0.23.0-3d007e03e-202106081720 --manifests /root/istio-1.10.0/manifests
```

或者，istio 原生安装，通过 pod annotation 来控制 sidecar 用 mosn

```yaml
template:
    metadata:
      annotations:
        proxy.istio.io/config: 'binary_path: /usr/local/bin/mosn'
        sidecar.istio.io/proxyImage: 192.168.8.88/mosnio/proxyv2:v0.23.0-3d007e03e-202106081720
        sidecar.istio.io/logLevel: debug
        sidecar.istio.io/componentLogLevel: 'misc:debug'
        status.sidecar.istio.io/port: '0'
```
