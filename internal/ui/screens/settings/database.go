package settings

import (
	"context"
	"fmt"
	"net/url"

	"awesomeProject/internal/config"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/ui/screens/shared"
	"awesomeProject/internal/ui/widgets"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newDatabaseSection(ctx context.Context, parent *gtk.Window, options DatabaseOptions) *gtk.Box {
	t := i18n.L()
	section := gtk.NewBox(gtk.OrientationVertical, 6)
	section.Append(widgets.NewSectionTitle(t.DatabaseLabel))
	current := options.Manager.Selection()
	drivers := []string{config.DriverSQLite, config.DriverPostgres}
	choice := gtk.NewDropDown(gtk.NewStringList([]string{"SQLite", "PostgreSQL"}), nil)
	if current.Driver == config.DriverPostgres {
		choice.SetSelected(1)
	}
	section.Append(choice)

	path := gtk.NewEntry()
	path.SetPlaceholderText(t.DatabaseSQLitePath)
	path.SetText(current.SQLitePath)
	section.Append(path)

	connection, err := config.PostgresConnectionFromURL(current.PostgresURL)
	if err != nil {
		connection = config.PostgresConnection{Host: "localhost", Port: "5432", Options: url.Values{}}
	}
	if connection.Port == "" {
		connection.Port = "5432"
	}
	host := databaseEntry(t.DatabaseHost, connection.Host)
	port := databaseEntry(t.DatabasePort, connection.Port)
	name := databaseEntry(t.DatabaseName, connection.Database)
	user := databaseEntry(t.DatabaseUser, connection.Username)
	password := databaseEntry(t.DatabasePassword, connection.Password)
	password.SetVisibility(false)
	passwordRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	passwordRow.Append(password)
	showPassword := gtk.NewButtonWithLabel(t.ShowPassword)
	showPassword.ConnectClicked(func() {
		visible := !password.Visibility()
		password.SetVisibility(visible)
		if visible {
			showPassword.SetLabel(t.HidePassword)
		} else {
			showPassword.SetLabel(t.ShowPassword)
		}
	})
	passwordRow.Append(showPassword)
	section.Append(host)
	section.Append(port)
	section.Append(name)
	section.Append(user)
	section.Append(passwordRow)

	updateSensitivity := func() {
		postgres := choice.Selected() == 1
		path.SetSensitive(!postgres)
		host.SetSensitive(postgres)
		port.SetSensitive(postgres)
		name.SetSensitive(postgres)
		user.SetSensitive(postgres)
		password.SetSensitive(postgres)
		showPassword.SetSensitive(postgres)
	}
	choice.NotifyProperty("selected", updateSensitivity)
	updateSensitivity()

	migrate := gtk.NewCheckButtonWithLabel(t.DatabaseMigrate)
	section.Append(migrate)
	help := gtk.NewLabel(t.DatabaseHelp)
	help.SetWrap(true)
	help.SetHAlign(gtk.AlignStart)
	help.AddCSSClass("dim-label")
	section.Append(help)
	message := widgets.NewMessageLabel()
	section.Append(message)
	apply := gtk.NewButtonWithLabel(t.DatabaseApply)
	apply.SetHAlign(gtk.AlignStart)
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
	section.Append(apply)
	return section
}

func databaseEntry(placeholder, text string) *gtk.Entry {
	entry := gtk.NewEntry()
	entry.SetPlaceholderText(placeholder)
	entry.SetText(text)
	return entry
}
