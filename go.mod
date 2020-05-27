module github.com/fjl/goleveldb-bench

go 1.14

require (
	github.com/aristanetworks/goarista v0.0.0-20200520141224-0f14e646773f
	github.com/gonum/blas v0.0.0-20181208220705-f22b278b28ac // indirect
	github.com/gonum/floats v0.0.0-20181209220543-c233463c7e82 // indirect
	github.com/gonum/internal v0.0.0-20181124074243-f884aa714029 // indirect
	github.com/gonum/lapack v0.0.0-20181123203213-e4cdc5a0bff9 // indirect
	github.com/gonum/matrix v0.0.0-20181209220409-c518dec07be9 // indirect
	github.com/gonum/stat v0.0.0-20181125101827-41a0da705a5b
	github.com/syndtr/goleveldb v1.0.0
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	gonum.org/v1/plot v0.7.0
)

replace github.com/syndtr/goleveldb => github.com/rjl493456442/goleveldb v0.0.0-20200526114915-f64b2e1ef944
