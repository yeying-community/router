package model

import "testing"

func TestSplitUserQuotaConsumption(t *testing.T) {
	tests := []struct {
		name              string
		total             int64
		dailyCapacity     int64
		emergencyCapacity int64
		dailyUnlimited    bool
		emergencyEnabled  bool
		wantDaily         int64
		wantEmergency     int64
	}{
		{
			name:          "daily only",
			total:         80,
			dailyCapacity: 100,
			wantDaily:     80,
			wantEmergency: 0,
		},
		{
			name:              "overflow to emergency",
			total:             130,
			dailyCapacity:     100,
			emergencyCapacity: 50,
			emergencyEnabled:  true,
			wantDaily:         100,
			wantEmergency:     30,
		},
		{
			name:              "emergency overdraw on late settlement",
			total:             170,
			dailyCapacity:     100,
			emergencyCapacity: 50,
			emergencyEnabled:  true,
			wantDaily:         100,
			wantEmergency:     70,
		},
		{
			name:             "unlimited daily bypasses emergency",
			total:            999,
			dailyUnlimited:   true,
			emergencyEnabled: true,
			wantDaily:        999,
			wantEmergency:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitUserQuotaConsumption(tt.total, tt.dailyCapacity, tt.emergencyCapacity, tt.dailyUnlimited, tt.emergencyEnabled)
			if got.DailyQuotaUsed != tt.wantDaily || got.EmergencyQuotaUsed != tt.wantEmergency {
				t.Fatalf("unexpected usage: got daily=%d emergency=%d, want daily=%d emergency=%d", got.DailyQuotaUsed, got.EmergencyQuotaUsed, tt.wantDaily, tt.wantEmergency)
			}
		})
	}
}
