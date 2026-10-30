package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/terminfo"
	"github.com/gliderlabs/ssh"
	"github.com/jmelahman/connections/game"
	"github.com/spf13/cobra"
)

var (
	port        string
	httpPort    string
	keyFile     string
	generateKey bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "connections-ssh",
		Short: "Play NYT Connections over SSH",
		Run: func(cmd *cobra.Command, args []string) {
			serve()
		},
	}

	rootCmd.Flags().StringVar(&port, "port", "2222", "Port to listen on for SSH")
	rootCmd.Flags().StringVar(&httpPort, "http-port", "8080", "Port to listen on for HTTP")
	rootCmd.Flags().StringVar(&keyFile, "key-file", "", "Path to SSH host key file")
	rootCmd.Flags().BoolVar(&generateKey, "generate-key", false, "Generate SSH key")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func serve() {
	ssh.Handle(func(s ssh.Session) {
		screen, err := NewSessionScreen(s)
		if err != nil {
			_, err := fmt.Fprintln(s, "Error creating screen session. Have you disabled pseudo-terminals (pty)?")
			if err != nil {
				log.Printf("Error writing to client: %v", err)
			}
			return
		}

		if err := game.RunWithScreen(screen); err != nil {
			log.Printf("Game error: %v", err)
		}
	})

	// Set up HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Connections over SSH</title>
  <style>
    :root{--bg:#0b0f12;--panel:#091217;--text:#d6deea;--muted:#7f8a98;--accent:#5ce1e6}
    html,body{height:100%%;margin:0;font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,"Liberation Mono",monospace;background:linear-gradient(180deg,#020409 0%%, #071018 100%%);color:var(--text)}
    .wrap{min-height:100%%;display:flex;align-items:center;justify-content:center;padding:3rem}
    .term{width:min(880px,94vw);background:linear-gradient(180deg,rgba(255,255,255,0.02),transparent);border-radius:12px;padding:28px 24px;box-shadow:0 12px 40px rgba(2,6,10,0.7);border:1px solid rgba(255,255,255,0.03)}
    .title{display:flex;align-items:center;gap:12px;margin-bottom:14px}
    .dots{display:flex;gap:8px}
    .dot{width:12px;height:12px;border-radius:50%%}
    .dot.r{background:#ff5f56}
    .dot.y{background:#ffbd2e}
    .dot.g{background:#27c93f}
    *{box-sizing: border-box}
    h1{font-size:16px;margin:0;color:var(--muted)}
    .screen{background:linear-gradient(180deg,#031017 0%%, #06121a 100%%);padding:20px;border-radius:8px;color:var(--text);min-height:220px;box-shadow:inset 0 1px 0 rgba(255,255,255,0.02)}
    .line{line-height:1.6;font-size:15px;white-space:pre-wrap}
    .prompt{color:var(--accent)}
    .cmd{display:inline-block;padding:6px 10px;border-radius:6px;background:rgba(255,255,255,0.01);margin-left:6px}
    .kbd{font-family:inherit;border:1px solid rgba(255,255,255,0.04);padding:4px 8px;border-radius:6px;background:rgba(0,0,0,0.25);font-size:13px}
    .toolbar{display:flex;gap:8px;align-items:center;margin-top:14px}
    button{background:transparent;border:1px solid rgba(255,255,255,0.04);padding:8px 12px;border-radius:8px;color:var(--text);cursor:pointer}
    button.primary{border-color:rgba(92,225,230,0.15);box-shadow:0 6px 18px rgba(92,225,230,0.03)}
    .cursor{display:inline-block;width:10px;height:18px;background:var(--text);margin-left:6px;vertical-align:middle;animation:blink 1s steps(2) infinite}
    @keyframes blink{50%%{opacity:0}}
    .muted{color:var(--muted);font-size:13px}
    .foot{margin-top:14px;color:var(--muted);font-size:13px}
  </style>
</head>
<body>
  <div class="wrap">
    <div class="term" role="main">
      <div class="title">
        <div class="dots"><span class="dot r"></span><span class="dot y"></span><span class="dot g"></span></div>
        <h1>ssh — connections</h1>
      </div>

      <div class="screen" aria-live="polite">
        <div class="line"><span class="muted">Run the following command to get started,</span></div>
        <div class="line" style="margin-top:10px"><span class="prompt">$</span><span class="cmd" id="ssh-cmd">ssh host</span><span class="cursor" aria-hidden="true"/></div>
        <div class="toolbar" style="margin-top:18px"><button id="copy">Copy command</button></div>
      </div>
    </div>
  </div>

  <script>
    (function(){
      const host = location.hostname || location.host || 'localhost';
      const sshCmdEl = document.getElementById('ssh-cmd');
      sshCmdEl.textContent = "ssh " + host;

      document.getElementById('copy').addEventListener('click', async ()=>{
        try{
          await navigator.clipboard.writeText(sshCmdEl.textContent);
          document.getElementById('copy').textContent = 'Copied ✓';
          setTimeout(()=>document.getElementById('copy').textContent='Copy command',1200);
        }catch(e){
          alert('Copy failed — select and copy manually.');
        }
      });

      // Accessibility: allow pressing Enter on Replace when focused
      document.getElementById('replace').addEventListener('keyup', (e)=>{ if(e.key==='Enter') document.getElementById('replace').click(); });
    })();
  </script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		if _, err := fmt.Fprint(w, html); err != nil {
			log.Printf("Error writing to client: %v", err)
		}
	})

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on :%s", httpPort)
		if err := http.ListenAndServe(":"+httpPort, nil); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	log.Printf("Starting SSH server on :%s", port)

	// Use provided key file or default to ~/.ssh/id_rsa
	hostKeyFile := keyFile
	if hostKeyFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		hostKeyFile = filepath.Join(home, ".ssh", "id_rsa")
	}
	if generateKey {
		if err := generatePrivateKey(hostKeyFile); err != nil {
			log.Fatal(err)
		}
	}

	log.Fatal(ssh.ListenAndServe(":"+port, nil, ssh.HostKeyFile(hostKeyFile)))
}

func generatePrivateKey(keyPath string) (err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	privateKeyFile, err := os.Create(keyPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, privateKeyFile.Close())
	}()

	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}
	return nil
}

func NewSessionScreen(s ssh.Session) (tcell.Screen, error) {
	pi, ch, ok := s.Pty()
	if !ok {
		return nil, errors.New("no pty requested")
	}
	ti, err := terminfo.LookupTerminfo(pi.Term)
	if err != nil {
		return nil, err
	}
	screen, err := tcell.NewTerminfoScreenFromTtyTerminfo(&tty{
		Session: s,
		size:    pi.Window,
		ch:      ch,
	}, ti)
	if err != nil {
		return nil, err
	}
	return screen, nil
}

type tty struct {
	ssh.Session
	size     ssh.Window
	ch       <-chan ssh.Window
	resizecb func()
	mu       sync.Mutex
}

func (t *tty) Start() error {
	go func() {
		for win := range t.ch {
			t.size = win
			t.notifyResize()
		}
	}()
	return nil
}

func (t *tty) Stop() error {
	return nil
}

func (t *tty) Drain() error {
	return nil
}

func (t *tty) WindowSize() (tcell.WindowSize, error) {
	return tcell.WindowSize{Width: t.size.Width, Height: t.size.Height}, nil
}

func (t *tty) NotifyResize(cb func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.resizecb = cb
}

func (t *tty) notifyResize() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resizecb != nil {
		t.resizecb()
	}
}
