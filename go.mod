module mosn.io/mosn

go 1.14

require (
	github.com/HdrHistogram/hdrhistogram-go v1.0.1 // indirect
	github.com/SkyAPM/go2sky v0.5.0
	github.com/TarsCloud/TarsGo v1.1.4
	github.com/alibaba/sentinel-golang v1.0.2-0.20210112133552-db6063eb263e
	github.com/apache/dubbo-go-hessian2 v1.7.0
	github.com/apache/thrift v0.13.0
	github.com/c2h5oh/datasize v0.0.0-20171227191756-4eba002a5eae
	github.com/cch123/supermonkey v1.0.1-0.20210420090843-d792ef7fb1d7
	github.com/cncf/udpa/go v0.0.0-20201120205902-5459f2c99403
	github.com/dchest/siphash v1.2.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/envoyproxy/go-control-plane v0.9.9-0.20210217033140-668b12f5399d
	github.com/ghodss/yaml v1.0.0
	github.com/go-chi/chi v4.1.0+incompatible
	github.com/go-resty/resty/v2 v2.6.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.3
	github.com/google/cel-go v0.5.1
	github.com/hashicorp/go-plugin v1.0.1
	github.com/json-iterator/go v1.1.10
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8 // indirect
	github.com/juju/testing v0.0.0-20191001232224-ce9dec17d28b // indirect
	github.com/lestrrat/go-jwx v0.0.0-20180221005942-b7d4802280ae
	github.com/lestrrat/go-pdebug v0.0.0-20180220043741-569c97477ae8 // indirect
	github.com/lyft/protoc-gen-validate v0.0.14
	github.com/miekg/dns v1.0.14
	github.com/mosn/binding v0.0.0-20200413092018-2b47bdb20a9f
	github.com/mosn/easygo v0.0.0-20201210062404-62796fdb3827
	github.com/mosn/registry v0.0.0-20210108061200-d7b63bc1904b
	github.com/opentracing/opentracing-go v1.1.0
	github.com/opentrx/seata-golang/v2 v2.0.0-rc2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/client_model v0.2.0
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0
	github.com/stretchr/testify v1.7.0
	github.com/tetratelabs/proxy-wasm-go-sdk v0.0.13
	github.com/trainyao/go-maglev v0.0.0-20200611125015-4c1ae64d96a8
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	github.com/urfave/cli v1.22.1
	github.com/valyala/fasthttp v0.0.0-20200605121233-ac51d598dc54
	github.com/valyala/fasttemplate v1.1.0
	github.com/wasmerio/wasmer-go v1.0.3
	go.uber.org/atomic v1.7.0
	go.uber.org/automaxprocs v1.3.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4
	golang.org/x/sys v0.0.0-20210510120138-977fb7262007
	golang.org/x/tools v0.1.4
	google.golang.org/genproto v0.0.0-20200806141610-86f49bd18e98
	google.golang.org/grpc v1.38.0
	google.golang.org/grpc/examples v0.0.0-20210714205950-0103ea2d6c98
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22 // indirect
	istio.io/api v0.0.0-20200227213531-891bf31f3c32
	k8s.io/klog v1.0.0
	mosn.io/api v0.0.0-20210714065837-5b4c2d66e70c
	mosn.io/pkg v0.0.0-20210604065522-6e8f5a087814
	mosn.io/proxy-wasm-go-host v0.0.0-20210312032409-2334f9cf62ec
	vimagination.zapto.org/byteio v0.0.0-20200222190125-d27cba0f0b10
	vimagination.zapto.org/memio v0.0.0-20200222190306-588ebc67b97d // indirect
)

replace github.com/envoyproxy/go-control-plane => github.com/envoyproxy/go-control-plane v0.9.4

replace github.com/dubbogo/gost => github.com/dubbogo/gost v1.9.9
