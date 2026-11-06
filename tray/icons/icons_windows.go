package icons

import "fmt"

func init() {
	for name, p := range windowsIcons() {
		ico, err := ICO(*p)
		if err != nil {
			panic(fmt.Sprintf("tray icon %s: %v", name, err))
		}
		*p = ico
	}
}
