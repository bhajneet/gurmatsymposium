// Gurmat Sangeet Broadcast System server.
//
// Serves the bundled index.html (Controller / Judge / OBS overlay views) over
// the local network so an iPad, a judge's laptop, and OBS Studio can all
// connect to one running instance. Everything the page needs (styling,
// fonts) is embedded in this binary — no internet connection is required at
// the event.
//
// Program state (the imported roster, which performer is active, and each
// performer's "done" flag) lives server-side in a JSON file next to the
// executable, not in browser localStorage — localStorage is scoped per
// browser *and* per port, so a random port each launch (or a separate
// browser process like OBS's embedded Chromium) would never see it. Every
// screen polls the server instead, which is also what keeps Controller,
// Judge, and OBS in sync with each other.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

//go:embed index.html style.css fonts
var webFS embed.FS

const (
	stateFileName    = "gurmat-state.json"
	passwordFileName = "gurmat-password.txt"
)

// defaultStateJSON is served when no state file has been saved yet.
const defaultStateJSON = `{"performers":[],"activeId":null,"fileName":""}`

var stateMu sync.Mutex

func main() {
	registerMimeTypes()

	dataDir := dataDirectory()
	if err := ensurePasswordFile(dataDir); err != nil {
		log.Printf("warning: could not create %s: %v", passwordFileName, err)
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	mux, err := buildMux(dataDir)
	if err != nil {
		log.Fatalf("could not prepare embedded assets: %v", err)
	}

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	ip := localIP()
	controllerURL := fmt.Sprintf("http://%s:%d/?screen=controller", ip, port)

	fmt.Println("==================================================================")
	fmt.Println("          Gurmat Sangeet Broadcast System")
	fmt.Println("==================================================================")
	fmt.Printf(" Controller (this iPad/computer or any device on this Wi-Fi):\n   http://%s:%d/?screen=controller\n\n", ip, port)
	fmt.Printf(" Judge / Presenter Display:\n   http://%s:%d/?screen=judge\n\n", ip, port)
	fmt.Printf(" OBS Browser Source (add on this computer, size 1920x1080):\n   http://localhost:%d/?screen=obs\n", port)
	fmt.Println("==================================================================")
	if passwordFileContents(dataDir) != "" {
		fmt.Printf(" Password protection: ON  (%s)\n", filepath.Join(dataDir, passwordFileName))
	} else {
		fmt.Printf(" Password protection: OFF (edit %s to set one)\n", passwordFileName)
	}
	fmt.Printf(" Program data saved to: %s\n", filepath.Join(dataDir, stateFileName))
	if csvName, ok := findSingleCSV(dataDir); ok {
		fmt.Printf(" Found %s next to the app -- will auto-import it if no roster is loaded yet.\n", csvName)
	}
	fmt.Println("==================================================================")
	fmt.Println(" Keep this window open for the rest of the event.")
	fmt.Println(" Press Ctrl+C (or close this window) to stop the server.")

	openBrowser(controllerURL)

	// Block until the user stops the server (Ctrl+C / closing the console
	// window). Deliberately no GUI window here — see README for why.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	fmt.Println("\nShutting down. Goodbye!")
}

func registerMimeTypes() {
	// Go's mime package derives extension types from the OS's mime registry
	// on some platforms, which may not know these; set them explicitly so
	// static assets always get correct Content-Type headers.
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".css", "text/css; charset=utf-8")
}

func buildMux(dataDir string) (*http.ServeMux, error) {
	mux := http.NewServeMux()

	indexHTML, err := webFS.ReadFile("index.html")
	if err != nil {
		return nil, err
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})

	styleCSS, err := webFS.ReadFile("style.css")
	if err != nil {
		return nil, err
	}
	mux.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(styleCSS)
	})

	fontsSub, err := fs.Sub(webFS, "fonts")
	if err != nil {
		return nil, err
	}
	mux.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.FS(fontsSub))))

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetState(dataDir, w, r)
		case http.MethodPost:
			handlePostState(dataDir, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/verify-password", func(w http.ResponseWriter, r *http.Request) {
		handleVerifyPassword(dataDir, w, r)
	})
	mux.HandleFunc("/api/auto-csv", func(w http.ResponseWriter, r *http.Request) {
		handleAutoCSV(dataDir, w, r)
	})

	return mux, nil
}

func handleGetState(dataDir string, w http.ResponseWriter, r *http.Request) {
	stateMu.Lock()
	data, err := os.ReadFile(filepath.Join(dataDir, stateFileName))
	stateMu.Unlock()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		_, _ = w.Write([]byte(defaultStateJSON))
		return
	}
	_, _ = w.Write(data)
}

func handlePostState(dataDir string, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB is far more than this ever needs
	if err != nil {
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}
	if !json.Valid(body) {
		http.Error(w, "request body is not valid JSON", http.StatusBadRequest)
		return
	}

	stateMu.Lock()
	err = writeFileAtomic(filepath.Join(dataDir, stateFileName), body)
	stateMu.Unlock()

	if err != nil {
		http.Error(w, "could not save state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleVerifyPassword(dataDir string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	expected := passwordFileContents(dataDir)
	ok := strings.TrimSpace(body.Password) == expected

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": ok})
}

// passwordFileContents returns the trimmed contents of the password file, or
// "" if the file doesn't exist or is empty (meaning the password is blank,
// so an empty submission from the prompt matches).
func passwordFileContents(dataDir string) string {
	data, err := os.ReadFile(filepath.Join(dataDir, passwordFileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// handleAutoCSV reports whether exactly one .csv file sits next to the
// executable and, if so, hands back its name and raw contents so the page
// can auto-import it on startup (only when no roster is loaded yet -- that
// check happens client-side). CSV parsing itself stays entirely in
// JavaScript (the same code path as the manual "Import CSV" button), this
// just does the filesystem discovery.
func handleAutoCSV(dataDir string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	name, ok := findSingleCSV(dataDir)
	if !ok {
		_ = json.NewEncoder(w).Encode(map[string]any{"available": false})
		return
	}

	data, err := os.ReadFile(filepath.Join(dataDir, name))
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"available": false})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"available": true,
		"filename":  name,
		"content":   string(data),
	})
}

// findSingleCSV returns the name of the one .csv file in dataDir, or
// ("", false) if there are zero or more than one -- ambiguous either way,
// so auto-import only ever fires when there's exactly one candidate.
func findSingleCSV(dataDir string) (string, bool) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return "", false
	}

	found := ""
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".csv") {
			found = e.Name()
			count++
		}
	}
	if count == 1 {
		return found, true
	}
	return "", false
}

// ensurePasswordFile creates an empty password file next to the executable
// if one doesn't already exist, so there's always a file an organizer can
// open and type a password into. Never overwrites an existing file (that
// would clobber a password someone already configured).
func ensurePasswordFile(dataDir string) error {
	path := filepath.Join(dataDir, passwordFileName)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte{}, 0644)
}

// writeFileAtomic writes via a temp file + rename so a crash or power loss
// mid-write can never leave gurmat-state.json truncated/corrupted.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// dataDirectory returns the directory the running executable lives in, so
// gurmat-state.json and gurmat-password.txt sit right next to it (works for
// a downloaded single-file binary with no install step). Falls back to the
// current working directory if the executable's path can't be resolved.
func dataDirectory() string {
	exe, err := os.Executable()
	if err != nil {
		if wd, werr := os.Getwd(); werr == nil {
			return wd
		}
		return "."
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

// localIP returns the machine's LAN IPv4 address (e.g. 192.168.1.50) so it
// can be shared with other devices on the same Wi-Fi. Falls back to
// "localhost" if none is found (e.g. no network connection).
func localIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	return "localhost"
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
