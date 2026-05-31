package main

import "github.com/TIANLI0/THRM/internal/guiapp"

// App keeps the Wails binding surface in package main while delegating implementation to internal/guiapp.
type App struct {
	*guiapp.App
}

func NewApp() *App {
	return &App{App: guiapp.New(newThemeManager())}
}
