package blocks

type BlockType string

// Events
const (
	WhenLaunch            BlockType = "cyberpi.cyberpi_when_launch"
	WhenButtonPress       BlockType = "cyberpi.cyberpi_when_button_press"
	WhenDirectionKeyPress BlockType = "cyberpi.cyberpi_when_direction_key_press"
)

// Statements
const (
	GetVolume BlockType = "cyberpi.cyberpi_get_volume"
	SetVolume BlockType = "cyberpi.cyberpi_set_volume"
	AddVolume BlockType = "cyberpi.cyberpi_add_volume"

	PlayBuzzerTone BlockType = "cyberpi.cyberpi_play_buzzer_tone"
	StopAudio      BlockType = "cyberpi.cyberpi_stop_audio"

	LEDShowSingleColor          BlockType = "cyberpi.cyberpi_led_show_single_with_color_2"
	LEDShowSingleColorFieldMenu BlockType = "cyberpi.cyberpi_led_show_single_with_color_2_fieldMenu_1_menu"
	MoveLED                     BlockType = "cyberpi.cyberpi_move_led"
	LEDOff                      BlockType = "cyberpi.cyberpi_led_off_2"
	LEDOffMenu                  BlockType = "cyberpi.cyberpi_led_off_2_fieldMenu_1_menu"
)

// Control
const (
	If          BlockType = "control_if"
	Wait        BlockType = "control_wait"
	RepeatUntil BlockType = "control_repeat_until"
)

// Operators
const (
	Equals   BlockType = "operator_equals"
	LessThan BlockType = "operator_lt"
)
