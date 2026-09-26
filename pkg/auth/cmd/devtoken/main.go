// Command devtoken is a dev-only helper: generates the dev RSA keypair and
// INTERNAL_TOKEN into .env, renders deploy/kong/kong.yml from its template,
// and mints JWTs for manual/scripted testing (D-27, D-28).
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/chonlatee11/boat-booking/pkg/auth"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: devtoken <keys|kong|token> [flags]")
	}
	var err error
	switch os.Args[1] {
	case "keys":
		err = cmdKeys()
	case "kong":
		err = cmdKong(os.Args[2:])
	case "token":
		err = cmdToken(os.Args[2:])
	default:
		fatal("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		fatal("%v", err)
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

const envPath = ".env"
const envExamplePath = ".env.example"

// cmdKeys makes .env from .env.example (never overwriting existing values),
// then generates an RSA keypair + INTERNAL_TOKEN if JWT_PRIVATE_KEY_B64 is
// still empty. Idempotent: never rotates existing keys.
func cmdKeys() error {
	env, err := loadEnvFile(envPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("devtoken keys: read .env: %w", err)
	}
	if env == nil {
		env = &envFile{values: map[string]string{}}
	}

	example, err := loadEnvFile(envExamplePath)
	if err != nil {
		return fmt.Errorf("devtoken keys: read .env.example: %w", err)
	}
	order := example.order
	for _, k := range order {
		if _, ok := env.values[k]; !ok {
			env.values[k] = example.values[k]
			env.order = append(env.order, k)
		}
	}

	if env.values["JWT_PRIVATE_KEY_B64"] == "" {
		privB64, pubB64, err := generateKeypairB64()
		if err != nil {
			return fmt.Errorf("devtoken keys: generate RSA keypair: %w", err)
		}
		internalToken, err := generateHexToken(32)
		if err != nil {
			return fmt.Errorf("devtoken keys: generate internal token: %w", err)
		}
		env.set("JWT_PRIVATE_KEY_B64", privB64)
		env.set("JWT_PUBLIC_KEY_B64", pubB64)
		env.set("INTERNAL_TOKEN", internalToken)
	}

	if err := env.writeFile(envPath, 0o600); err != nil {
		return fmt.Errorf("devtoken keys: write .env: %w", err)
	}
	fmt.Println("devtoken: .env ready")
	return nil
}

func cmdKong(args []string) error {
	fs := flag.NewFlagSet("kong", flag.ExitOnError)
	in := fs.String("in", "deploy/kong/kong.yml.tmpl", "template path")
	out := fs.String("out", "deploy/kong/kong.yml", "output path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	env, err := loadEnvFile(envPath)
	if err != nil {
		return fmt.Errorf("devtoken kong: read .env: %w (run make dev-keys first)", err)
	}
	issuer := env.values["JWT_ISSUER"]
	if issuer == "" {
		issuer = "boatbooking-dev"
	}
	pubB64 := env.values["JWT_PUBLIC_KEY_B64"]
	if pubB64 == "" {
		return fmt.Errorf("devtoken kong: JWT_PUBLIC_KEY_B64 is empty (run make dev-keys first)")
	}
	pub, err := auth.ParsePublicKeyB64(pubB64)
	if err != nil {
		return fmt.Errorf("devtoken kong: %w", err)
	}
	pubPEMBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return fmt.Errorf("devtoken kong: marshal public key: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubPEMBytes})

	// Indent every PEM line by 10 spaces to sit under the YAML `|` block in
	// the template (consumers[0].jwt_secrets[0].rsa_public_key).
	var indented strings.Builder
	sc := bufio.NewScanner(bytes.NewReader(pubPEM))
	for sc.Scan() {
		indented.WriteString("          ")
		indented.WriteString(sc.Text())
		indented.WriteString("\n")
	}

	tmplBytes, err := os.ReadFile(*in)
	if err != nil {
		return fmt.Errorf("devtoken kong: read template: %w", err)
	}
	tmpl, err := template.New("kong").Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("devtoken kong: parse template: %w", err)
	}
	var buf bytes.Buffer
	data := struct {
		Issuer       string
		PublicKeyPEM string
	}{Issuer: issuer, PublicKeyPEM: strings.TrimRight(indented.String(), "\n")}
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("devtoken kong: render template: %w", err)
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil { //nolint:gosec // rendered kong.yml is not a secret file
		return fmt.Errorf("devtoken kong: write %s: %w", *out, err)
	}
	fmt.Printf("devtoken: rendered %s\n", *out)
	return nil
}

func cmdToken(args []string) error {
	fs := flag.NewFlagSet("token", flag.ExitOnError)
	sub := fs.String("sub", "00000000-0000-0000-0000-000000000001", "user id")
	operator := fs.String("operator", "00000000-0000-0000-0000-0000000000a1", "operator id")
	role := fs.String("role", "pier_admin", "role")
	pierIDs := fs.String("pier-ids", "", "comma-separated pier uuids (empty = no pier scoping)")
	kind := fs.String("kind", "access", "token kind: access|refresh")
	age := fs.Duration("age", 0, "shift issuance back by this duration (produces an expired token)")
	foreignKey := fs.Bool("foreign-key", false, "sign with a freshly generated RSA key instead of the dev key (negative test)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var tokenKind auth.Kind
	switch *kind {
	case "access":
		tokenKind = auth.KindAccess
	case "refresh":
		tokenKind = auth.KindRefresh
	default:
		return fmt.Errorf("devtoken token: invalid -kind %q", *kind)
	}

	env, err := loadEnvFile(envPath)
	if err != nil {
		return fmt.Errorf("devtoken token: read .env: %w (run make dev-keys first)", err)
	}
	issuer := env.values["JWT_ISSUER"]
	if issuer == "" {
		issuer = "boatbooking-dev"
	}

	var priv *rsa.PrivateKey
	if *foreignKey {
		priv, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("devtoken token: generate foreign key: %w", err)
		}
	} else {
		privB64 := env.values["JWT_PRIVATE_KEY_B64"]
		if privB64 == "" {
			return fmt.Errorf("devtoken token: JWT_PRIVATE_KEY_B64 is empty (run make dev-keys first)")
		}
		priv, err = auth.ParsePrivateKeyB64(privB64)
		if err != nil {
			return fmt.Errorf("devtoken token: %w", err)
		}
	}

	var pierIDList []string
	if *pierIDs != "" {
		pierIDList = strings.Split(*pierIDs, ",")
	}

	issuerObj := auth.NewIssuer(priv, issuer)
	now := time.Now().Add(-*age)
	tok, err := issuerObj.Issue(auth.Claims{
		UserID:     *sub,
		OperatorID: *operator,
		Role:       *role,
		PierIDs:    pierIDList,
		Kind:       tokenKind,
	}, now)
	if err != nil {
		return fmt.Errorf("devtoken token: issue: %w", err)
	}
	fmt.Println(tok)
	return nil
}

func generateKeypairB64() (privB64, pubB64 string, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return "", "", err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	return base64.StdEncoding.EncodeToString(privPEM), base64.StdEncoding.EncodeToString(pubPEM), nil
}

func generateHexToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// envFile is a minimal ordered KEY=value store, preserving comments is not
// needed here — devtoken only reads/writes machine-managed env files.
type envFile struct {
	values map[string]string
	order  []string
}

func (e *envFile) set(key, val string) {
	if _, ok := e.values[key]; !ok {
		e.order = append(e.order, key)
	}
	e.values[key] = val
}

func loadEnvFile(path string) (*envFile, error) {
	f, err := os.Open(path) //nolint:gosec // dev CLI reads only its own hardcoded/flag-supplied local paths (.env, .env.example, kong template)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only fd close, nothing actionable on error

	e := &envFile{values: map[string]string{}}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		e.values[key] = val
		e.order = append(e.order, key)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *envFile) writeFile(path string, mode os.FileMode) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec // dev CLI writes only its own hardcoded local .env path
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, k := range e.order {
		if _, err := fmt.Fprintf(w, "%s=%s\n", k, e.values[k]); err != nil {
			_ = f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
