// Deliberately risky dependencies to exercise the pipeline's supply-chain
// gates (Trivy image scan, govulncheck, Socket). Blank imports are enough to
// link each module into the binary's build info, which is where the image
// scanner reads the dependency inventory. gogs.io/gogs exposes no importable
// package (everything is under internal/), so it lives in go.mod only.
// Remove this file after the scanner demo.
package main

import (
	_ "github.com/casdoor/casdoor/util"
	_ "github.com/dhax/go-base/logging"
	_ "golang.org/x/crypto/ssh"
)
