package cmd

import (
	"net"
	"testing"
)

func TestGetIpAddress(t *testing.T) {
	tests := []struct {
		domain      string
		wantIP      string
		shouldError bool
	}{
		{"<Domain>", "<IP address>", false},
		{"not-found.domain", "N/A", true},
	}

	for _, tt := range tests {
		gotIP, _ := getIpAddress(tt.domain)

		if tt.shouldError && gotIP != "N/A" {
			t.Errorf("Expected N/A for %s, but got %s", tt.domain, gotIP)
		} else if !tt.shouldError && gotIP == "N/A" {
			t.Errorf("Expected vailid IP for %s, but got N/A", tt.domain)
		}
	}
}

func TestCheckPortOpen(t *testing.T) {
	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer listener.Close()

	if !checkPortOpen("localhost", 8081) {
		t.Errorf("Expected port 8081 to be open, but checkPortOpen() returned false")
	}

	if checkPortOpen("localhost", 9999) {
		t.Errorf("Expected port 9999 to be closed, but checkPortOpen() returned true")
	}
}

func TestColorizeStatus(t *testing.T) {
	if colorizeStatus("active") != colorGreen+"active"+colorReset {
		t.Errorf("Color for 'active' is incorrect")
	}
	if colorizeStatus("deactive") != colorRed+"deactive"+colorReset {
		t.Errorf("Color for 'deactive' is incorrect")
	}
}

func TestColorizeCloud(t *testing.T) {
	if colorizeCloud("AWS") != colorYellow+"AWS"+colorReset {
		t.Errorf("Color for 'AWS' is incorrect")
	}
	if colorizeCloud("Azure") != colorCyan+"Azure"+colorReset {
		t.Errorf("Color for 'Azure' is incorrect")
	}
	if colorizeCloud("GCP") != colorBlue+"GCP"+colorReset {
		t.Errorf("Color for 'GCP' is incorrect")
	}
	if colorizeCloud("unknown") != "unknown" {
		t.Errorf("Color for 'unknown' is incorrect")
	}
}
