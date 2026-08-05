package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mdp/qrterminal/v3"
	"rsc.io/qr"
	"vcode/internal/bridge"
	"vcode/internal/runtime"
)

// bridgeCommand manages the durable local computer target. The execution
// engine remains the normal Vcode Agent; this command only owns the bridge
// lifecycle and its approved project registry.
func bridgeCommand(args []string) int {
	if len(args) == 0 {
		bridgeUsage()
		return 2
	}
	root, rest := bridgeRootFlag(args)
	if len(rest) == 0 {
		bridgeUsage()
		return 2
	}
	s, err := bridge.Open(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vcode bridge:", err)
		return 1
	}
	switch rest[0] {
	case "status":
		t := s.Snapshot()
		fmt.Printf("%s\t%s\t%s\t%s\n", t.ID, t.Name, t.Status, t.Kind)
		return 0
	case "start":
		return bridgeStart(s, rest[1:])
	case "stop":
		if err := s.SetStatus(runtime.TargetOffline); err != nil {
			fmt.Fprintln(os.Stderr, "vcode bridge:", err)
			return 1
		}
		fmt.Println("Vcode Bridge stopped")
		return 0
	case "pair":
		p, err := s.NewPairing()
		if err != nil {
			fmt.Fprintln(os.Stderr, "vcode bridge:", err)
			return 1
		}
		relay, token := s.RelayConfig()
		if err := bridge.PublishPairing(relay, token, p); err != nil {
			fmt.Fprintf(os.Stderr, "vcode bridge: pairing was created locally but relay was not updated: %v\n", err)
		}
		if os.Getenv("VCODE_PAIRING_LEGACY_OUTPUT") == "" {
			return printPairingSummary(s, relay, p)
		}
		fmt.Printf("配对地址: %s\n", bridge.PairURL(relay, p))
		fmt.Printf("设备: %s\n配对码: %s\n有效期: %s\n", s.Snapshot().Name, p.Code, p.ExpiresAt.Local().Format("2006-01-02 15:04:05"))
		return 0
	case "configure":
		return bridgeConfigure(s, rest[1:])
	case "project":
		return bridgeProjectCommand(s, rest[1:])
	default:
		bridgeUsage()
		return 2
	}
}

func printPairingSummary(s *bridge.Store, relay string, p runtime.PairingRequest) int {
	pairURL := bridge.PairURL(relay, p)
	fmt.Printf("Device: %s\nPairing code: %s\nExpires: %s\n\nScan this QR code in the Vcode app:\n", s.Snapshot().Name, p.Code, p.ExpiresAt.Local().Format("2006-01-02 15:04:05"))
	qrterminal.GenerateHalfBlock(pairURL, qr.M, os.Stdout)
	fmt.Printf("Pairing URL: %s\n", pairURL)
	return 0
}

func bridgeRootFlag(args []string) (string, []string) {
	root := ""
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--root=") {
			root = strings.TrimPrefix(args[i], "--root=")
			continue
		}
		rest = append(rest, args[i])
	}
	return root, rest
}

func bridgeStart(s *bridge.Store, args []string) int {
	configuredRelay, configuredToken := s.RelayConfig()
	fs := flag.NewFlagSet("bridge start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	relay := fs.String("relay", configuredRelay, "cloud relay WebSocket URL")
	token := fs.String("token", configuredToken, "computer Bridge token")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*relay) != configuredRelay || strings.TrimSpace(*token) != configuredToken {
		if err := s.SetRelayConfig(*relay, *token); err != nil {
			fmt.Fprintln(os.Stderr, "vcode bridge:", err)
			return 1
		}
	}
	if err := s.SetStatus(runtime.TargetOnline); err != nil {
		fmt.Fprintln(os.Stderr, "vcode bridge:", err)
		return 1
	}
	t := s.Snapshot()
	fmt.Printf("Vcode Bridge online: %s (%s)\n", t.Name, t.ID)
	fmt.Println("已登记项目可由手机端选择；按 Ctrl+C 停止。")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if strings.TrimSpace(*relay) != "" {
		client := &bridge.Client{RelayURL: *relay, Token: *token, Store: s, Output: func(message bridge.Message) {
			if message.Type == bridge.MessageError {
				fmt.Printf("bridge relay error: %s\n", string(message.Payload))
				return
			}
			if message.Type == bridge.MessageRuntimeEvent {
				fmt.Printf("task %s event\n", message.TaskID)
			}
		}}
		go func() {
			if err := client.Run(ctx); err != nil && ctx.Err() == nil {
				fmt.Fprintln(os.Stderr, "vcode bridge relay:", err)
			}
		}()
	}
	<-sig
	signal.Stop(sig)
	_ = s.SetStatus(runtime.TargetOffline)
	fmt.Println("Vcode Bridge offline")
	return 0
}

func bridgeProjectCommand(s *bridge.Store, args []string) int {
	if len(args) == 0 {
		bridgeUsage()
		return 2
	}
	switch args[0] {
	case "list":
		for _, p := range s.ProjectsSnapshot() {
			mode := "rw"
			if p.ReadOnly {
				mode = "ro"
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", p.ID, p.Name, mode, p.Root)
		}
		return 0
	case "add":
		fs := flag.NewFlagSet("bridge project add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "project name")
		path := fs.String("path", "", "project directory")
		readOnly := fs.Bool("read-only", false, "register as read-only")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if *path == "" && len(fs.Args()) == 1 {
			*path = fs.Args()[0]
		}
		p, err := s.AddProject(*name, *path, *readOnly)
		if err != nil {
			fmt.Fprintln(os.Stderr, "vcode bridge:", err)
			return 1
		}
		fmt.Printf("registered %s (%s)\n", p.Name, p.ID)
		return 0
	case "remove":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: vcode bridge project remove ID")
			return 2
		}
		if err := s.RemoveProject(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "vcode bridge:", err)
			return 1
		}
		return 0
	default:
		bridgeUsage()
		return 2
	}
}

func bridgeConfigure(s *bridge.Store, args []string) int {
	fs := flag.NewFlagSet("bridge configure", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	relay := fs.String("relay", "", "cloud relay WebSocket URL")
	token := fs.String("token", "", "computer Bridge token")
	tokenFile := fs.String("token-file", "", "read the computer Bridge token from a protected file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*tokenFile) != "" {
		b, err := os.ReadFile(*tokenFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "vcode bridge:", err)
			return 1
		}
		value := strings.TrimSpace(string(b))
		if i := strings.Index(value, "VCODE_BRIDGE_TOKEN="); i >= 0 {
			value = value[i+len("VCODE_BRIDGE_TOKEN="):]
			if j := strings.IndexAny(value, " \r\n"); j >= 0 {
				value = value[:j]
			}
		}
		*token = strings.TrimSpace(value)
	}
	if strings.TrimSpace(*relay) == "" || strings.TrimSpace(*token) == "" {
		fmt.Fprintln(os.Stderr, "usage: vcode bridge configure --relay wss://HOST/api/bridge/connect (--token TOKEN | --token-file PATH)")
		return 2
	}
	if err := s.SetRelayConfig(*relay, *token); err != nil {
		fmt.Fprintln(os.Stderr, "vcode bridge:", err)
		return 1
	}
	fmt.Println("Vcode Bridge relay configured")
	return 0
}

func bridgeUsage() {
	fmt.Print(`Usage:
  vcode bridge start [--root PATH] [--relay wss://HOST/api/bridge/connect] [--token TOKEN]
  vcode bridge stop [--root PATH]
  vcode bridge status [--root PATH]
  vcode bridge pair [--root PATH]
  vcode bridge configure --relay wss://HOST/api/bridge/connect (--token TOKEN | --token-file PATH)
  vcode bridge project add --name NAME --path PATH [--read-only]
  vcode bridge project list
  vcode bridge project remove ID
`)
}
