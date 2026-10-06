#!/bin/sh
# Sends one request to the fuku API with server.auth.token from the effective config.
# The header reaches curl on stdin, so the token is in no argv, environment variable, file or output.
set -eu

fail() { echo "api.sh: $1" >&2; exit "${2:-1}"; }

config=
if [ "${1:-}" = --config ]; then
	[ $# -ge 2 ] || fail "--config needs a path" 2
	config=$2
	shift 2
fi
[ $# -eq 2 ] || fail "usage: api.sh [--config <path>] GET|POST <url>" 2
method=$1
url=$2

case $method in GET | POST) ;; *) fail "method must be GET or POST" 2 ;; esac

case $url in
http://127.0.0.1:*) rest=${url#http://127.0.0.1:} ;;
http://localhost:*) rest=${url#http://localhost:} ;;
http://\[::1\]:*) rest=${url#http://\[::1\]:} ;;
*) fail "url must start with http://127.0.0.1:<port>/, http://localhost:<port>/ or http://[::1]:<port>/" 2 ;;
esac
port=${rest%%/*}
case $port in "" | *[!0-9]*) fail "url must name a numeric port followed by a path" 2 ;; esac
[ "$rest" != "$port" ] || fail "url must name a numeric port followed by a path" 2

prog='
function fail(m) { print "api.sh: " m | "cat 1>&2"; exit 1 }
function cannot() { fail("server.auth.token has a form this script cannot read") }
function readable(f,   l, r) { r = (getline l < f); close(f); return r >= 0 }
function watched(n) {
	return K[1] == "server" && (n == 1 || K[2] == "auth" && (n == 2 || K[3] == "token" && n == 3))
}
function parse(s,   c) {
	c = substr(s, 1, 1)
	if (c == "\"" || c == "\047") {
		if (!match(s, c == "\"" ? "^\"[^\"\\\\]*\"" : "^\047([^\047]|\047\047)*\047")) return 0
		KEY = substr(s, 2, RLENGTH - 2); s = substr(s, RLENGTH + 1)
		if (c == "\047") gsub("\047\047", "\047", KEY)
		if (!match(s, /^[ \t]*:([ \t]|$)/)) return 0
	} else {
		if (c ~ /[][{}&*!|>%@`#?:,-]/ || !match(s, /:([ \t]|$)/)) return 0
		KEY = substr(s, 1, RSTART - 1); sub(/[ \t]+$/, "", KEY)
		if (KEY ~ /[ \t]#/) return 0
	}
	VAL = substr(s, RSTART + RLENGTH); sub(/^[ \t]+/, "", VAL)
	return KEY != "<<"
}
function value(n,   v) {
	v = VAL
	if (v ~ /^#/ || v ~ /^[&!][^ \t]*([ \t]+#.*)?$/) v = ""
	DEEP = v == "" ? -1 : D[n]
	if (!watched(n)) return
	if (v == "") PEND = n
	else settle(n, v)
}
function settle(n, v,   q, i, rest) {
	q = substr(v, 1, 1)
	if (q != "\"" && q != "\047") { sub(/[ \t]+#.*/, "", v); sub(/[ \t]+$/, "", v) }
	if (v ~ /^(null|Null|NULL|~)?$/) { S = "null"; return }
	if (v ~ /^[|>&*!{[%@`]/) cannot()
	if (n < 3) { S = "null"; return }
	if (q == "\"") {
		i = index(substr(v, 2), "\"")
		if (!i) cannot()
		rest = substr(v, i + 2); v = substr(v, 2, i - 1)
		if (v ~ /\\/) cannot()
	} else if (q == "\047") {
		if (!match(v, "^\047([^\047]|\047\047)*\047")) cannot()
		rest = substr(v, RLENGTH + 1); v = substr(v, 2, RLENGTH - 2)
		gsub("\047\047", "\047", v)
	}
	if (rest !~ /^([ \t]+(#.*)?)?$/) cannot()
	S = "set"; V = v
}
function scan(f,   line, d, s, top, inside, n, seen) {
	S = ""; PEND = 0; DEEP = -1; top = ""; inside = 0; n = 0; seen = 0
	while ((getline line < f) > 0) {
		sub(/\r$/, "", line)
		if (line ~ /^[ \t]*(#|$)/) continue
		match(line, /^ */); d = RLENGTH; s = substr(line, d + 1)
		if (d == 0 && !seen && s ~ /^---[ \t]*(#.*)?$/) continue
		seen = 1
		if (inside && DEEP >= 0 && d > DEEP) cannot()
		if (PEND && d > D[PEND] && PEND == 3) cannot()
		if (PEND && d <= D[PEND]) settle(PEND, "")
		PEND = 0
		if (d == 0 && s ~ /^-([ \t]|$)/ && top != "" && !inside) continue
		if (d > 0 && top == "") cannot()
		if (d > 0 && !inside) continue
		if (!parse(s)) cannot()
		if (d == 0) { top = KEY; inside = KEY == "server"; n = 0 }
		while (n && D[n] >= d) n--
		D[++n] = d; K[n] = KEY
		if (inside) value(n)
	}
	if (PEND) settle(PEND, "")
	close(f)
}
BEGIN {
	cfg = ENVIRON["API_SH_CONFIG"]
	if (cfg != "") {
		if (!readable(cfg)) fail("cannot read " cfg)
		scan(cfg)
	} else {
		b = "fuku.yaml"; if (!readable(b)) b = "fuku.yml"
		if (!readable(b)) fail("no fuku.yaml or fuku.yml here, run from the project root")
		o = "fuku.override.yaml"; if (!readable(o)) o = "fuku.override.yml"
		S = ""; if (readable(o)) scan(o)
		if (S == "") scan(b)
	}
	if (S != "set" || V == "") fail("no server.auth.token in the effective config")
	if (emit) print "Authorization: Bearer " V
}'

API_SH_CONFIG="$config" awk -v emit=0 "$prog"
API_SH_CONFIG="$config" awk -v emit=1 "$prog" |
	curl -q -g -sS --noproxy '*' -m 10 -w '\n%{http_code}\n' -X "$method" -H @- "$url"
