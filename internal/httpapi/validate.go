package httpapi

import "regexp"

// Input validation to keep operator-supplied values out of remote root shell
// commands (project/domain/email are interpolated into systemctl/certbot) and
// out of the remote tokens file (frp username/token). Allowlist-only.
var (
	reProject  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	reHostname = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	reEmail    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$`)
	rePath     = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)
	reFrpName  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	reFrpToken = regexp.MustCompile(`^[A-Za-z0-9._@:+=-]{1,128}$`)
)

func validProject(s string) bool  { return reProject.MatchString(s) }
func validHostname(s string) bool { return s == "" || reHostname.MatchString(s) }
func validEmail(s string) bool    { return s == "" || reEmail.MatchString(s) }
func validPath(s string) bool     { return rePath.MatchString(s) }
func validFrpName(s string) bool  { return reFrpName.MatchString(s) }

// validFrpToken allows a broader set but still forbids newlines, '=' and
// whitespace so it cannot inject extra lines into the tokens file.
func validFrpToken(s string) bool { return reFrpToken.MatchString(s) }
