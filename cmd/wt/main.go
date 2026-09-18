package main

import (
	"github.com/suzuki-shunsuke/cobra-util/cobrautil"
	"github.com/suzuki-shunsuke/wt/pkg/cli"
)

var version = ""

func main() {
	cobrautil.Main("wt", version, cli.Run)
}
