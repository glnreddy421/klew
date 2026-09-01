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
	m.Append(buildViewMenu(app))
	m.Append(buildTerminalMenu(app))
	m.Append(buildToolsMenu(app))
	m.Append(menu.WindowMenu())
	m.Append(buildHelpMenu(app))
	return m
}

func buildViewMenu(app *App) *menu.MenuItem {
	view := menu.NewMenu()

	view.AddText("Focus Search", keys.CmdOrCtrl("k"), func(*menu.CallbackData) {
		emitMenuEvent(app, "focus-search")
	})
	view.AddSeparator()
	view.AddText("Settings…", keys.CmdOrCtrl(","), func(*menu.CallbackData) {
		emitMenuEvent(app, "settings")
	})

	return menu.SubMenu("View", view)
}

func buildTerminalMenu(app *App) *menu.MenuItem {
	terminal := menu.NewMenu()

	terminal.AddText("Show Terminal", keys.CmdOrCtrl("`"), func(*menu.CallbackData) {
		emitMenuEvent(app, "terminal")
	})
	terminal.AddText("Show Live Logs", keys.Combo("l", keys.CmdOrCtrlKey, keys.ShiftKey), func(*menu.CallbackData) {
		emitMenuEvent(app, "live-logs")
	})
	terminal.AddText("Split Panes", keys.Combo("`", keys.CmdOrCtrlKey, keys.ShiftKey), func(*menu.CallbackData) {
		emitMenuEvent(app, "console-split")
	})

	return menu.SubMenu("Terminal", terminal)
}

func buildToolsMenu(app *App) *menu.MenuItem {
	tools := menu.NewMenu()

	tools.AddText("Sync Kubeconfig", keys.CmdOrCtrl("r"), func(*menu.CallbackData) {
		app.SyncCluster()
	})
	tools.AddText("New Window", keys.CmdOrCtrl("n"), func(*menu.CallbackData) {
		_ = app.OpenNewWindow(OpenWindowOptions{})
	})
	tools.AddSeparator()
	tools.AddText("Open Kubeconfig Folder…", nil, func(*menu.CallbackData) {
		app.OpenKubeconfigDir()
	})

	return menu.SubMenu("Tools", tools)
}

func buildHelpMenu(app *App) *menu.MenuItem {
	help := menu.NewMenu()

	help.AddText("Klew Help", nil, func(*menu.CallbackData) {
		emitMenuEvent(app, "help")
	})

	return menu.SubMenu("Help", help)
}

func emitMenuEvent(app *App, name string) {
	if app == nil || app.ctx == nil {
		return
	}
	runtime.EventsEmit(app.ctx, "menu:"+name)
}
