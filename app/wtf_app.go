package app

import (
	"log"
	"time"

	"github.com/gdamore/tcell"
	"github.com/olebedev/config"
	"github.com/radovskyb/watcher"
	"github.com/rivo/tview"
	"github.com/wtfutil/wtf/cfg"
	"github.com/wtfutil/wtf/utils"
	"github.com/wtfutil/wtf/wtf"
)

// WtfApp is the container for a collection of widgets that are all constructed from a single
// configuration file and displayed together
type WtfApp struct {
	app            *tview.Application
	config         *config.Config
	configFilePath string
	display        *Display
	focusTracker   FocusTracker
	isCustomConfig bool
	pages          *tview.Pages
	validator      *ModuleValidator
	widgets        []wtf.Wtfable
}

// NewWtfApp creates and returns an instance of WtfApp
func NewWtfApp(app *tview.Application, config *config.Config, configFilePath string, isCustom bool) *WtfApp {
	wtfApp := WtfApp{
		app:            app,
		config:         config,
		configFilePath: configFilePath,
		isCustomConfig: isCustom,
		pages:          tview.NewPages(),
	}

	wtfApp.app.SetInputCapture(wtfApp.keyboardIntercept)
	wtfApp.widgets = MakeWidgets(wtfApp.app, wtfApp.pages, wtfApp.config)
	wtfApp.display = NewDisplay(wtfApp.widgets, wtfApp.config)
	wtfApp.focusTracker = NewFocusTracker(wtfApp.app, wtfApp.widgets, wtfApp.config)
	wtfApp.validator = NewModuleValidator()

	wtfApp.pages.AddPage("grid", wtfApp.display.Grid, true, true)
	wtfApp.app.SetRoot(wtfApp.pages, true)

	wtfApp.validator.Validate(wtfApp.widgets)

	return &wtfApp
}

/* -------------------- Exported Functions -------------------- */

// App returns the *tview.Application instance
func (wtfApp *WtfApp) App() *tview.Application {
	return wtfApp.app
}

// Start initializes the app
func (wtfApp *WtfApp) Start() {
	wtfApp.scheduleWidgets()
	go wtfApp.watchForConfigChanges()
}

// Stop kills all the currently-running widgets in this app
func (wtfApp *WtfApp) Stop() {
	wtfApp.stopAllWidgets()
}

/* -------------------- Unexported Functions -------------------- */

func (wtfApp *WtfApp) stopAllWidgets() {
	for _, widget := range wtfApp.widgets {
		widget.Stop()
	}
}

func (wtfApp *WtfApp) keyboardIntercept(event *tcell.EventKey) *tcell.EventKey {
	// These keys are global keys used by the app. Widgets should not implement these keys
	switch event.Key() {
	case tcell.KeyCtrlR:
		wtfApp.refreshAllWidgets()
		return nil
	case tcell.KeyTab:
		wtfApp.focusTracker.Next()
	case tcell.KeyBacktab:
		wtfApp.focusTracker.Prev()
		return nil
	case tcell.KeyEsc:
		wtfApp.focusTracker.None()
	}

	// Checks to see if any widget has been assigned the pressed key as its focus key
	if wtfApp.focusTracker.FocusOn(string(event.Rune())) {
		return nil
	}

	// If no specific widget has focus, then allow the key presses to fall through to the app
	if !wtfApp.focusTracker.IsFocused {
		switch string(event.Rune()) {
		case "/":
			return nil
		}
	}

	return event
}

func (wtfApp *WtfApp) refreshAllWidgets() {
	for _, widget := range wtfApp.widgets {
		go widget.Refresh()
	}
}

func (wtfApp *WtfApp) scheduleWidgets() {
	for _, widget := range wtfApp.widgets {
		go Schedule(widget)
	}
}

func (wtfApp *WtfApp) watchForConfigChanges() {
	watch := watcher.New()

	// Notify write events
	watch.FilterOps(watcher.Write)

	go func() {
		for {
			select {
			case <-watch.Event:
				wtfApp.Stop()

				config := cfg.LoadWtfConfigFile(wtfApp.configFilePath, wtfApp.isCustomConfig)

				newApp := NewWtfApp(wtfApp.app, config, wtfApp.configFilePath, wtfApp.isCustomConfig)
				newApp.Start()
			case err := <-watch.Error:
				log.Fatalln(err)
			case <-watch.Closed:
				return
			}
		}
	}()

	// Watch config file for changes.
	absPath, _ := utils.ExpandHomeDir(wtfApp.configFilePath)
	if err := watch.Add(absPath); err != nil {
		log.Fatalln(err)
	}

	// Start the watching process - it'll check for changes every 100ms.
	if err := watch.Start(time.Millisecond * 100); err != nil {
		log.Fatalln(err)
	}
}
