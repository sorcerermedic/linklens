package analytics

import "strings"

const (
	DeviceDesktop = "desktop"
	DeviceMobile  = "mobile"
	DeviceTablet  = "tablet"
	DeviceBot     = "bot"
	DeviceUnknown = "unknown"
)

var botMarkers = []string{"bot", "crawler", "spider", "slurp", "curl/", "wget/", "python-requests", "go-http-client", "headlesschrome", "facebookexternalhit", "preview"}

func ClassifyDevice(userAgent string) string {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return DeviceUnknown
	}
	for _, m := range botMarkers {
		if strings.Contains(ua, m) {
			return DeviceBot
		}
	}
	switch {
	case strings.Contains(ua, "ipad"),
		strings.Contains(ua, "tablet"),
		strings.Contains(ua, "kindle"),
		strings.Contains(ua, "silk/"),
		strings.Contains(ua, "android") && !strings.Contains(ua, "mobile"):
		return DeviceTablet
	case strings.Contains(ua, "mobile"),
		strings.Contains(ua, "iphone"),
		strings.Contains(ua, "ipod"),
		strings.Contains(ua, "android"),
		strings.Contains(ua, "windows phone"),
		strings.Contains(ua, "blackberry"):
		return DeviceMobile
	case strings.Contains(ua, "windows"),
		strings.Contains(ua, "macintosh"),
		strings.Contains(ua, "x11"),
		strings.Contains(ua, "linux"),
		strings.Contains(ua, "cros"):
		return DeviceDesktop
	}
	return DeviceUnknown
}
