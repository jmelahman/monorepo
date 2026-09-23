package rules

import "testing"

// packageBody wraps a single command in a package() function.
func packageBody(cmd string) map[string]string {
	return map[string]string{"PKGBUILD": pkgbuildWith("", "package() {\n  "+cmd+"\n}")}
}

func TestSetuidNumericMode(t *testing.T) {
	cases := []struct {
		mode string
		want bool
	}{
		// setuid / setgid set, whatever the spelling
		{"4755", true},
		{"04755", true},
		{"2755", true},
		{"02755", true},
		{"6755", true},
		{"06755", true},
		{"7755", true}, // setuid+setgid+sticky
		{"04000", true},
		{"2000", true},
		// no setuid/setgid bit
		{"755", false},
		{"0755", false},
		{"644", false},
		{"0644", false},
		{"1777", false}, // sticky only
		{"01777", false},
		{"0", false},
		// not a numeric mode at all
		{"", false},
		{"u+s", false},
		{"a+x", false},
		{"-R", false},
		{"--reference=4755", false},
		{"$pkgdir/usr/bin/demo", false},
		{"4855", false}, // 8 is not an octal digit
		{"-4755", false},
		{"+4755", false},
	}
	for _, tc := range cases {
		if got := setuidNumericMode(tc.mode); got != tc.want {
			t.Errorf("setuidNumericMode(%q) = %v, want %v", tc.mode, got, tc.want)
		}
	}
}

func TestPB403NumericModeDetection(t *testing.T) {
	flagged := []string{
		// Newly caught: the leading digit is not 4/2/6 but the bit is set.
		`chmod 04755 "$pkgdir/usr/bin/demo"`,
		`chmod 02755 "$pkgdir/usr/bin/demo"`,
		`chmod 06755 "$pkgdir/usr/bin/demo"`,
		`chmod 7755 "$pkgdir/usr/bin/demo"`,
		`install -m 04755 demo "$pkgdir/usr/bin/demo"`,
		`install --mode=02755 demo "$pkgdir/usr/bin/demo"`,
		// Already caught before: no regression.
		`chmod 4755 "$pkgdir/usr/bin/demo"`,
		`chmod 2755 "$pkgdir/usr/bin/demo"`,
		`chmod 6755 "$pkgdir/usr/bin/demo"`,
		`chmod u+s "$pkgdir/usr/bin/demo"`,
		`chmod g+s "$pkgdir/usr/bin/demo"`,
		`install -Dm4755 demo "$pkgdir/usr/bin/demo"`,
	}
	for _, cmd := range flagged {
		t.Run(cmd, func(t *testing.T) {
			expectRule(t, "PB403", packageBody(cmd))
		})
	}

	clean := []string{
		`chmod 0755 "$pkgdir/usr/bin/demo"`,
		`chmod 755 "$pkgdir/usr/bin/demo"`,
		`chmod 1777 "$pkgdir/var/tmp/demo"`,
		`chmod 01777 "$pkgdir/var/tmp/demo"`,
		`chmod a+x "$pkgdir/usr/bin/demo"`,
		`install -m 0644 demo "$pkgdir/usr/share/demo/demo"`,
		`install -Dm755 demo "$pkgdir/usr/bin/demo"`,
	}
	for _, cmd := range clean {
		t.Run(cmd, func(t *testing.T) {
			expectNoRule(t, "PB403", packageBody(cmd))
		})
	}
}
