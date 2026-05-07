module github.com/iamhectordev/hector

go 1.26

require (
	github.com/doron-cohen/klee v0.0.0
	github.com/oklog/ulid/v2 v2.1.1
	github.com/sourcegraph/conc v0.3.0
	github.com/stretchr/testify v1.11.1
	github.com/urfave/cli/v3 v3.8.0
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lmittmann/tint v1.1.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	gopkg.in/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/doron-cohen/klee => ../klee
