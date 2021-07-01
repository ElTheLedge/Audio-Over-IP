package main

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var windowHeight float32 = 128
var windowLength float32 = 384

var connStatuses = []string{ //Status Numbers
	"Idle",              //0
	"Connected",         //1
	"Connection failed", //2
	"Connecting...",     //3
	"Connection lost",   //4
}

func startGUI(closeGUI *bool) {
	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme())

	w := a.NewWindow("Audio over IP")
	w.CenterOnScreen()

	ipInput := widget.NewEntry()
	ipInput.SetPlaceHolder("127.0.0.1")

	portInput := widget.NewEntry()
	portInput.SetPlaceHolder("4040")

	var connStatusNum *int = new(int)
	connStatusNumBindingInt := binding.BindInt(connStatusNum)
	go func() {
		for {
			connStatusNumBindingIntRefresher(&connStatusNumBindingInt, &connStatusNum)
			time.Sleep(100 * time.Millisecond)
		} //Put this in loop incase the function crashes
	}()
	connStatusStr := binding.NewString()

	var cancelAudio context.CancelFunc
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "IP address", Widget: ipInput},
			{Text: "Port", Widget: portInput},
		},
	}
	form.SubmitText = "Connect"
	form.CancelText = "Disconnect"

	var disableFormInputs = func() {
		ipInput.DisableableWidget.Disable()
		portInput.DisableableWidget.Disable()
	}
	var enableFormInputs = func() {
		ipInput.DisableableWidget.Enable()
		portInput.DisableableWidget.Enable()
	}

	//Form refresher loop
	go func() {
		var disconnect bool = false
		for {
			time.Sleep(100 * time.Millisecond)
			switch *connStatusNum {
			case 1:
				form.OnSubmit = nil
				form.OnCancel = func() {
					disconnect = true
					*connStatusNum = 0
					enableFormInputs()
				}
				disableFormInputs()
				form.Refresh()
			case 2:
				enableFormInputs()
			case 3:
				disableFormInputs()
			case 4, 0:
				enableFormInputs()
				form.OnCancel = nil
				form.OnSubmit = func() {
					disconnect = false
					connStatusNum = audioStartup(ipInput.Text+":"+portInput.Text, &disconnect)
					disableFormInputs()
				}
				form.Refresh()
			}
		}
	}()

	connStatusNumBindingInt.AddListener(binding.NewDataListener(func() {
		num, _ := connStatusNumBindingInt.Get()
		connStatusStr.Set(connStatuses[num])

	}))

	connStatusLabel := widget.NewLabel("Status:")
	connStatusLabel.TextStyle.Bold = true
	connStatus := widget.NewLabelWithData(connStatusStr)
	connStatusLayout := container.New(layout.NewHBoxLayout(), connStatusLabel, connStatus)

	w.Resize(fyne.NewSize(windowLength, windowHeight))
	w.SetContent(container.NewVBox(
		form,
		connStatusLayout,
	))
	w.SetOnClosed(func() {
		if cancelAudio != nil {
			cancelAudio()
		}
	})
	w.ShowAndRun()
}

func connStatusNumBindingIntRefresher(a *binding.ExternalInt, b **int) {
	var tempNum = **b
	for {
		time.Sleep(100 * time.Millisecond)
		if tempNum != **b {
			(*a).Set(**b)
			tempNum = **b
		}
	}
}
