module cocoq

go 1.26.1

require (
	github.com/elazarl/goproxy v1.8.4-0.20260429163546-e493e1c4c552
	github.com/elazarl/goproxy/ext v0.0.0-20260327201742-eeb2adb11cb5
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.10.2
	k8s.io/apimachinery v0.35.3
)

require (
	github.com/andybalholm/brotli v1.0.6 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/refraction-networking/utls v1.8.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)

replace github.com/elazarl/goproxy => github.com/realab/goproxy v0.0.0-20260430040556-8e0b09c7fc98

replace github.com/elazarl/goproxy/ext => github.com/realab/goproxy/ext v0.0.0-20260430040556-8e0b09c7fc98
