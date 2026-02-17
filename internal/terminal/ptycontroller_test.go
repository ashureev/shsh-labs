package terminal

import (
	"testing"
	"time"
)

func TestDefaultPTYConfig(t *testing.T) {
	config := DefaultPTYConfig()

	if config.TypingSpeed != 75*time.Millisecond {
		t.Errorf("expected TypingSpeed 75ms, got %v", config.TypingSpeed)
	}

	if config.JitterMax != 25*time.Millisecond {
		t.Errorf("expected JitterMax 25ms, got %v", config.JitterMax)
	}

	if config.ThinkPause != 500*time.Millisecond {
		t.Errorf("expected ThinkPause 500ms, got %v", config.ThinkPause)
	}
}

func TestNewPTYController(t *testing.T) {
	config := DefaultPTYConfig()
	controller := NewPTYController(nil, config, nil)

	if controller == nil {
		t.Fatal("expected controller to be created")
	}

	if controller.config.TypingSpeed != config.TypingSpeed {
		t.Errorf("expected TypingSpeed %v, got %v", config.TypingSpeed, controller.config.TypingSpeed)
	}
}

func TestPTYController_SetTypingSpeed(t *testing.T) {
	config := DefaultPTYConfig()
	controller := NewPTYController(nil, config, nil)

	newSpeed := 100 * time.Millisecond
	controller.SetTypingSpeed(newSpeed)

	if controller.GetConfig().TypingSpeed != newSpeed {
		t.Errorf("expected TypingSpeed %v, got %v", newSpeed, controller.GetConfig().TypingSpeed)
	}
}

func TestPTYController_GetConfig(t *testing.T) {
	config := PTYConfig{
		TypingSpeed:      50 * time.Millisecond,
		JitterMax:        10 * time.Millisecond,
		ThinkPause:       200 * time.Millisecond,
		PunctuationPause: 50 * time.Millisecond,
	}
	controller := NewPTYController(nil, config, nil)

	retrievedConfig := controller.GetConfig()

	if retrievedConfig.TypingSpeed != config.TypingSpeed {
		t.Errorf("expected TypingSpeed %v, got %v", config.TypingSpeed, retrievedConfig.TypingSpeed)
	}

	if retrievedConfig.JitterMax != config.JitterMax {
		t.Errorf("expected JitterMax %v, got %v", config.JitterMax, retrievedConfig.JitterMax)
	}
}
