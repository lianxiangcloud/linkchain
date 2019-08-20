package xcrypto

// #cgo LDFLAGS: -L./libs_linux  -lxcrypto -lboost_system -lboost_filesystem -lboost_thread -lboost_date_time -lboost_system -lboost_regex -lboost_chrono -lsodium -lssl -lcrypto  -lstdc++ -ldl -lpthread
import "C"
