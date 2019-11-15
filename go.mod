module github.com/lianxiangcloud/linkchain

go 1.12

replace (
	github.com/NebulousLabs/go-upnp => github.com/lianxiangcloud/go-upnp v0.0.0-20190905032046-65768e0b268c
	github.com/go-interpreter/wagon => github.com/xunleichain/wagon v0.5.3
	gopkg.in/sourcemap.v1 => github.com/go-sourcemap/sourcemap v1.0.5
)

require (
	bou.ke/monkey v1.0.1 // indirect
	github.com/BurntSushi/toml v0.3.1
	github.com/NebulousLabs/fastrand v0.0.0-20181203155948-6fb6489aac4e // indirect
	github.com/NebulousLabs/go-upnp v0.0.0-00010101000000-000000000000
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/aristanetworks/goarista v0.0.0-20190704150520-f44d68189fd7
	github.com/boltdb/bolt v1.3.1
	github.com/bouk/monkey v1.0.1
	github.com/btcsuite/btcd v0.0.0-20190629003639-c26ffa870fd8
	github.com/btcsuite/btcutil v0.0.0-20190425235716-9e5f4b9a998d
	github.com/cespare/cp v1.1.1
	github.com/davecgh/go-spew v1.1.1
	github.com/dgraph-io/badger v1.6.0
	github.com/ebuchman/fail-test v0.0.0-20170303061230-95f809107225
	github.com/fatih/color v1.7.0
	github.com/fortytw2/leaktest v1.3.0
	github.com/go-kit/kit v0.8.0
	github.com/go-stack/stack v1.8.0
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.2
	github.com/golang/snappy v0.0.1
	github.com/hashicorp/golang-lru v0.5.1
	github.com/influxdata/influxdb v1.7.7
	github.com/jmhodges/levigo v1.0.0
	github.com/mattn/go-colorable v0.1.2
	github.com/pborman/uuid v1.2.0
	github.com/peterh/liner v1.1.0
	github.com/pkg/errors v0.8.1
	github.com/prashantv/gostub v1.0.0
	github.com/prometheus/client_golang v1.0.0
	github.com/rjeczalik/notify v0.9.2
	github.com/robertkrimen/otto v0.0.0-20180617131154-15f95af6e78d
	github.com/rs/cors v1.6.0
	github.com/smartystreets/goconvey v0.0.0-20190731233626-505e41936337
	github.com/spaolacci/murmur3 v1.1.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.3.0
	github.com/syndtr/goleveldb v1.0.0
	github.com/xunleichain/tc-wasm v0.3.5
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190712062909-fae7ac547cb7
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127
	gopkg.in/fatih/set.v0 v0.1.0
	gopkg.in/h2non/gock.v1 v1.0.15
	gopkg.in/karalabe/cookiejar.v2 v2.0.0-20150724131613-8dcd6a7f4951
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce
	gopkg.in/sourcemap.v1 v1.0.0-00010101000000-000000000000 // indirect
)
