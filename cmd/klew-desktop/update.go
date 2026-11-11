package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/glnreddy421/klew/v2/internal/update"
	"github.com/glnreddy421/klew/v2/internal/version"
)

// CheckForUpdate compares the running build to the latest GitHub release.
// It never downloads or installs — Settings / Help use the result to guide the user.
func (a *App) CheckForUpdate() update.Result {
	ctx := context.Background()
	if a != nil && a.ctx != nil {
		ctx = a.ctx
	}
	return update.Check(ctx, version.Version, "", "")
}

func (a *App) checkForUpdateDialog() {
	if a == nil || a.ctx == nil {
		return
	}
	res := a.CheckForUpdate()
	title := "Check for Updates"
	msg := res.Message
	if res.Error != "" {
		msg = res.Message + "\n\n" + res.Error
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   title,
			Message: msg,
		})
		return
	}
	if !res.UpdateAvailable {
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   title,
			Message: msg,
		})
		return
	}
	if res.Channel == update.ChannelHomebrew && res.UpgradeCommand != "" {
		msg += "\n\n" + res.UpgradeCommand
	}
	choice, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.InfoDialog,
		Title:         "Update available",
		Message:       msg,
		Buttons:       []string{"Open release", "Later"},
		DefaultButton: "Open release",
		CancelButton:  "Later",
	})
	if err != nil {
		return
	}
	if choice == "Open release" && res.ReleaseURL != "" {
		runtime.BrowserOpenURL(a.ctx, res.ReleaseURL)
	}
}
