package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/mcpcli"
	"github.com/glnreddy421/klew/internal/version"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// "Klew mcp" serves MCP over stdio. It returns before any Wails or app
	// setup, so it opens no window and adds no Dock icon, and stdout carries
	// only the protocol.
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		os.Exit(mcpcli.Main(mcpcli.AppProg, os.Args[2:], os.Stdout, os.Stderr))
	}

	contextName := flag.String("context", "", "initial kube context for this window")
	namespace := flag.String("namespace", "", "initial namespace for this window")
	kubeconfig := flag.String("kubeconfig", "", "kubeconfig path override for this window")
	flag.Parse()

	kube.BootstrapLoginShellEnv()
	kube.InitNetworkProxyFromEnv()

	app := NewApp(bootOptions{
		Context:    *contextName,
		Namespace:  *namespace,
		Kubeconfig: *kubeconfig,
	})
	err := wails.Run(&options.App{
		Title:     "Klew",
		Width:     1280,
		Height:    860,
		MinWidth:  960,
		MinHeight: 640,
		Menu:      buildApplicationMenu(app),
		Mac: &mac.Options{
			Appearance:  mac.NSAppearanceNameDarkAqua,
			DisableZoom: false,
			About: &mac.AboutInfo{
				Title:   "Klew",
				Message: fmt.Sprintf("Everything your cluster is saying. One screen.\n\nVersion %s", version.Version),
			},
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 17, B: 23, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
