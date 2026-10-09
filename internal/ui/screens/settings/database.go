package settings

import (
	"awesomeProject/internal/ui/layout"
	"context"
	"fmt"
	"net/url"

	"awesomeProject/internal/config"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/screens/shared"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newDatabaseSection(ctx context.Context, parent *gtk.Window, options DatabaseOptions, views ...*layout.View) *gtk.Box {
	view := layout.Use("database-settings", views...)

	t := i18n.L()
	section := layout.Get[*gtk.Box](view, "section")
	_ = layout.Configure[*gtk.Label](view, "object_1165", func(w *gtk.Label) { w.SetLabel(t.DatabaseLabel) })
	current := options.Manager.Selection()
	drivers := []string{config.DriverSQLite, config.DriverPostgres}
	choice := layout.Configure[*gtk.DropDown](view, "choice", func(w *gtk.DropDown) { w.SetModel(gtk.NewStringList([]string{"SQLite", "PostgreSQL"})) })
	if current.Driver == config.DriverPostgres {
		choice.SetSelected(1)
	}

	path := layout.Get[*adw.EntryRow](view, "path")
	path.SetTitle(t.DatabaseSQLitePath)
	path.SetText(current.SQLitePath)

	connection, err := config.PostgresConnectionFromURL(current.PostgresURL)
	if err != nil {
		connection = config.PostgresConnection{Host: "localhost", Port: "5432", Options: url.Values{}}
	}
	if connection.Port == "" {
		connection.Port = "5432"
	}
	postgresGroup := layout.Get[*adw.PreferencesGroup](view, "postgresGroup")
	host := databaseEntry(t.DatabaseHost, connection.Host, layout.Scope(view, "postgresGroup_slot_1"))
	port := databaseEntry(t.DatabasePort, connection.Port, layout.Scope(view, "postgresGroup_slot_2"))
	port.SetInputPurpose(gtk.InputPurposeNumber)
	name := databaseEntry(t.DatabaseName, connection.Database, layout.Scope(view, "postgresGroup_slot_3"))
	user := databaseEntry(t.DatabaseUser, connection.Username, layout.Scope(view, "postgresGroup_slot_4"))
	password := layout.Get[*adw.PasswordEntryRow](view, "password")
	password.SetTitle(t.DatabasePassword)
	password.SetText(connection.Password)

	updateSensitivity := func() {
		postgres := choice.Selected() == 1
		path.SetVisible(!postgres)
		postgresGroup.SetVisible(postgres)
	}
	choice.NotifyProperty("selected", updateSensitivity)
	updateSensitivity()

	migrate := layout.Configure[*gtk.CheckButton](view, "migrate", func(w *gtk.CheckButton) { w.SetLabel(t.DatabaseMigrate) })
	_ = layout.Configure[*gtk.Label](view, "help", func(w *gtk.Label) { w.SetLabel(t.DatabaseHelp) })

	message := layout.Get[*gtk.Label](view, "message")

	apply := layout.Configure[*gtk.Button](view, "apply", func(w *gtk.Button) { w.SetLabel(t.DatabaseApply) })

	apply.ConnectClicked(func() {
		index := choice.Selected()
		if int(index) >= len(drivers) {
			return
		}
		next := config.DatabaseSelection{Driver: drivers[index], SQLitePath: path.Text()}
		if next.Driver == config.DriverPostgres {
			next.PostgresURL, err = (config.PostgresConnection{
				Host: host.Text(), Port: port.Text(), Database: name.Text(),
				Username: user.Text(), Password: password.Text(), Options: connection.Options,
			}).URL()
			if err != nil {
				message.SetLabel(fmt.Sprintf("%s: %v", t.SaveError, err))
				message.SetVisible(true)
				return
			}
		}
		copyData := migrate.Active()
		apply.SetSensitive(false)
		if parent != nil {
			parent.SetSensitive(false)
		}
		message.SetLabel("…")
		message.SetVisible(true)
		var switchErr error
		shared.RunAsyncUI(func() { switchErr = options.Manager.Switch(ctx, next, copyData) }, func() {
			if switchErr != nil {
				message.SetLabel(fmt.Sprintf("%s: %v", t.SaveError, switchErr))
				apply.SetSensitive(true)
				if parent != nil {
					parent.SetSensitive(true)
				}
				return
			}
			if options.OnSwitched != nil {
				options.OnSwitched()
			}
		})
	})

	return section
}

func databaseEntry(title, text string, views ...*layout.View) *adw.EntryRow {
	view := layout.Use("database-entry", views...)

	entry := layout.Get[*adw.EntryRow](view, "entry")
	entry.SetTitle(title)
	entry.SetText(text)
	return entry
}
