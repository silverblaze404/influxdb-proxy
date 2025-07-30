package banner

import "fmt"

const custom_logo = `
██ ███    ██ ███████ ██      ██    ██ ██   ██ ██████  ██████      ██████  ██████   ██████  ██   ██ ██    ██ 
██ ████   ██ ██      ██      ██    ██  ██ ██  ██   ██ ██   ██     ██   ██ ██   ██ ██    ██  ██ ██   ██  ██  
██ ██ ██  ██ █████   ██      ██    ██   ███   ██   ██ ██████      ██████  ██████  ██    ██   ███     ████   
██ ██  ██ ██ ██      ██      ██    ██  ██ ██  ██   ██ ██   ██     ██      ██   ██ ██    ██  ██ ██     ██    
██ ██   ████ ██      ███████  ██████  ██   ██ ██████  ██████      ██      ██   ██  ██████  ██   ██    ██    
                                                                                                            
                                    High-Performance InfluxDB Proxy Server                     
                                    Version %s                                                                     
`

// Display shows the InfluxDB Proxy trademark banner with version information
func Display(version string) {

	// Add some spacing for clean presentation
	fmt.Print("\n\n")

	// Display the trademark logo in color
	fmt.Printf("\033[36m"+custom_logo+"\033[0m\n", version) // Cyan color for the logo

	// Add some spacing and additional info
	fmt.Printf("\033[33m%s\033[0m\n", "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") // Yellow separator
	fmt.Printf("\033[32m%s\033[0m\n", "🚀 Starting InfluxDB Proxy with advanced query filtering")
	fmt.Printf("\033[33m%s\033[0m\n\n", "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") // Yellow separator
}

// GetVersion returns a simple version string without the banner
func GetVersion(appName, version string) string {
	return fmt.Sprintf("%s version %s", appName, version)
}
