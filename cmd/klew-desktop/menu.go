package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func buildApplicationMenu(app *App) *menu.Menu {
	m := menu.NewMenu()
	m.Append(menu.AppMenu())
	m.Append(menu.EditMenu())
	m.Append(buildToolsMenu(app))
	m.Append(menu.WindowMenu())
	return m
}

func buildToolsMenu(app *App) *menu.MenuItem {
	tools := menu.NewMenu()

	tools.AddText("Focus Search", keys.CmdOrCtrl("k"), func(*menu.CallbackData) {
		emitMenuEvent(app, "focus-search")
	})
	tools.AddText("Sync Kubeconfig", keys.CmdOrCtrl("r"), func(*menu.CallbackData) {
		app.SyncCluster()
	})
	tools.AddText("New Window", keys.CmdOrCtrl("n"), func(*menu.CallbackData) {
		_ = app.OpenNewWindow(OpenWindowOptions{})
	})
	tools.AddSeparator()
	tools.AddText("Settings…", keys.CmdOrCtrl(","), func(*menu.CallbackData) {
		emitMenuEvent(app, "settings")
	})
	tools.AddText("Help", nil, func(*menu.CallbackData) {
		emitMenuEvent(app, "help")
	})
	tools.AddSeparator()
	tools.AddText("Open Kubeconfig Folder…", nil, func(*menu.CallbackData) {
		app.OpenKubeconfigDir()
	})

	return menu.SubMenu("Tools", tools)
}

func emitMenuEvent(app *App, name string) {
	if app == nil || app.ctx == nil {
		return
	}
	runtime.EventsEmit(app.ctx, "menu:"+name)
}
