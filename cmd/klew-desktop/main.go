package main

import (
	"embed"
	"flag"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	contextName := flag.String("context", "", "initial kube context for this window")
	namespace := flag.String("namespace", "", "initial namespace for this window")
	kubeconfig := flag.String("kubeconfig", "", "kubeconfig path override for this window")
	flag.Parse()

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
