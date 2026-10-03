package i18n

// Message ID constants — single source of truth for all message keys.
// These IDs map directly to keys defined in the locale JSON files under /locales/.
const (
	// Main Menu
	MsgMainMenuGreeting          = "main_menu_greeting"
	MsgMainMenuAlreadyRegistered = "main_menu_already_registered"
	MsgMainMenuEnterFullName     = "main_menu_enter_full_name"
	MsgMainMenuMustRegisterFirst = "main_menu_must_register_first"
	MsgMainMenuMustRegisterRadar = "main_menu_must_register_for_radar"
	MsgMainMenuTooManyWrongInput = "main_menu_too_many_wrong_input"
	MsgMainMenuUnrecognizedChoice = "main_menu_unrecognized_choice"

	// Register
	MsgRegisterNameCannotBeEmpty = "register_name_cannot_be_empty"
	MsgRegisterAlreadyRegistered = "register_already_registered"
	MsgRegisterSuccess           = "register_success"

	// Order
	MsgOrderMenu           = "order_menu"
	MsgOrderInvalidChoice  = "order_invalid_choice"
	MsgOrderAccountNotFound = "order_account_not_found"
	MsgOrderSuccess        = "order_success"

	// Radar Saham Menu
	MsgRadarSahamMenu          = "radar_saham_menu"
	MsgRadarAccountNotRegistered = "radar_account_not_registered"
	MsgRadarInvalidChoice      = "radar_invalid_choice"

	// Subsector
	MsgSubsectorMenu        = "subsector_menu"
	MsgSubsectorCustomPrompt = "subsector_custom_prompt"
	MsgSubsectorFetchError  = "subsector_fetch_error"

	// Watchlist
	MsgWatchlistEmpty         = "watchlist_empty"
	MsgWatchlistFetchError    = "watchlist_fetch_error"
	MsgWatchlistAddSuccess    = "watchlist_add_success"
	MsgWatchlistAddFetchError = "watchlist_add_fetch_error"
	MsgWatchlistUserNotFound  = "watchlist_user_not_found"
	MsgWatchlistInvalidTicker = "watchlist_invalid_ticker_add"

	// Manual Ticker
	MsgManualTickerPrompt     = "manual_ticker_prompt"
	MsgManualInvalidTicker    = "manual_invalid_ticker"
	MsgManualTickerFetchError = "manual_ticker_fetch_error"

	// Drill-down
	MsgDrillDownFetchError = "drilldown_fetch_error"

	// Agent / Stock Inquiry
	MsgAgentError       = "agent_error"
	MsgAgentEmptyResult = "agent_empty_result"
	MsgAgentFooter      = "agent_footer"

	// Opt-in Notifications
	MsgOptInPrompt           = "optin_prompt"
	MsgOptInSubsectorEnabled = "optin_subsector_enabled"
	MsgOptInWatchlistEnabled = "optin_watchlist_enabled"
	MsgOptInDisabled         = "optin_disabled"
	MsgOptInInvalidChoice    = "optin_invalid_choice"

	// Memory Control Commands
	MsgMemoryEmpty         = "memory_empty"
	MsgMemoryForgetInvalid = "memory_forget_invalid"
	MsgMemoryForgotConfirm = "memory_forgot_confirm"
	MsgChatHistoryDeleted  = "chat_history_deleted"
	MsgAllMemoryDeleted    = "all_memory_deleted"
)
