package autostart

import "testing"

func TestCommandLine(t *testing.T) {
	cases := []struct {
		name      string
		exePath   string
		minimized bool
		want      string
	}{
		{"plain path", `C:\Program Files\Aion Theory Sync\aion-theory-sync-gui.exe`, false,
			`"C:\Program Files\Aion Theory Sync\aion-theory-sync-gui.exe"`},
		{"minimized", `C:\Program Files\Aion Theory Sync\aion-theory-sync-gui.exe`, true,
			`"C:\Program Files\Aion Theory Sync\aion-theory-sync-gui.exe" --minimized`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := commandLine(c.exePath, c.minimized); got != c.want {
				t.Fatalf("commandLine(%q, %v) = %q, want %q", c.exePath, c.minimized, got, c.want)
			}
		})
	}
}

func TestFake_satisfiesTheInterfaceAndRecordsWhatWasEnabled(t *testing.T) {
	var a Autostart = &Fake{}

	enabled, err := a.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled: %v", err)
	}
	if enabled {
		t.Fatal("IsEnabled = true before anything was enabled")
	}

	if err := a.Enable(`C:\app\aion-theory-sync-gui.exe`, true); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	enabled, err = a.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled: %v", err)
	}
	if !enabled {
		t.Fatal("IsEnabled = false after Enable")
	}

	fake := a.(*Fake)
	if fake.ExePath != `C:\app\aion-theory-sync-gui.exe` || !fake.Minimized {
		t.Fatalf("Fake = %+v, want the exe path and Minimized recorded", fake)
	}

	if err := a.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	enabled, err = a.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled: %v", err)
	}
	if enabled {
		t.Fatal("IsEnabled = true after Disable")
	}
}

func TestFake_errorsAreReturnedWithoutRecordingAnything(t *testing.T) {
	boom := errFake("boom")
	f := &Fake{EnableErr: boom}

	if err := f.Enable("exe", false); err != boom {
		t.Fatalf("Enable err = %v, want %v", err, boom)
	}
	if f.Enabled {
		t.Fatal("Enabled = true despite Enable failing")
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }
