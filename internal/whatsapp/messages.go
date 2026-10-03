package whatsapp_gateway

// message_keys.go re-exports i18n message IDs from pkg/i18n as unexported
// package-level constants, so the service implementation stays clean
// and does not need to import pkg/i18n directly.

import pkgi18n "go-rich-buddy-platform/pkg/i18n"

// Compile-time aliases — zero runtime cost.
const (
	msgMainMenuGreeting           = pkgi18n.MsgMainMenuGreeting
	msgMainMenuAlreadyRegistered  = pkgi18n.MsgMainMenuAlreadyRegistered
	msgMainMenuEnterFullName      = pkgi18n.MsgMainMenuEnterFullName
	msgMainMenuMustRegisterFirst  = pkgi18n.MsgMainMenuMustRegisterFirst
	msgMainMenuMustRegisterRadar  = pkgi18n.MsgMainMenuMustRegisterRadar
	msgMainMenuTooManyWrongInput  = pkgi18n.MsgMainMenuTooManyWrongInput
	msgMainMenuUnrecognizedChoice = pkgi18n.MsgMainMenuUnrecognizedChoice

	msgRegisterNameCannotBeEmpty = pkgi18n.MsgRegisterNameCannotBeEmpty
	msgRegisterAlreadyRegistered = pkgi18n.MsgRegisterAlreadyRegistered
	msgRegisterSuccess           = pkgi18n.MsgRegisterSuccess

	msgOrderMenu            = pkgi18n.MsgOrderMenu
	msgOrderInvalidChoice   = pkgi18n.MsgOrderInvalidChoice
	msgOrderAccountNotFound = pkgi18n.MsgOrderAccountNotFound
	msgOrderSuccess         = pkgi18n.MsgOrderSuccess

	msgRadarSahamMenu            = pkgi18n.MsgRadarSahamMenu
	msgRadarAccountNotRegistered = pkgi18n.MsgRadarAccountNotRegistered
	msgRadarInvalidChoice        = pkgi18n.MsgRadarInvalidChoice

	msgSubsectorMenu         = pkgi18n.MsgSubsectorMenu
	msgSubsectorCustomPrompt = pkgi18n.MsgSubsectorCustomPrompt
	msgSubsectorFetchError   = pkgi18n.MsgSubsectorFetchError

	msgWatchlistEmpty         = pkgi18n.MsgWatchlistEmpty
	msgWatchlistFetchError    = pkgi18n.MsgWatchlistFetchError
	msgWatchlistAddSuccess    = pkgi18n.MsgWatchlistAddSuccess
	msgWatchlistAddFetchError = pkgi18n.MsgWatchlistAddFetchError
	msgWatchlistUserNotFound  = pkgi18n.MsgWatchlistUserNotFound
	msgWatchlistInvalidTicker = pkgi18n.MsgWatchlistInvalidTicker

	msgManualTickerPrompt     = pkgi18n.MsgManualTickerPrompt
	msgManualInvalidTicker    = pkgi18n.MsgManualInvalidTicker
	msgManualTickerFetchError = pkgi18n.MsgManualTickerFetchError

	msgDrillDownFetchError    = pkgi18n.MsgDrillDownFetchError
	msgDrillDownInvalidTicker = pkgi18n.MsgDrillDownInvalidTicker

	msgAgentError       = pkgi18n.MsgAgentError
	msgAgentEmptyResult = pkgi18n.MsgAgentEmptyResult
	msgAgentFooter      = pkgi18n.MsgAgentFooter

	msgOptInPrompt           = pkgi18n.MsgOptInPrompt
	msgOptInSubsectorEnabled = pkgi18n.MsgOptInSubsectorEnabled
	msgOptInWatchlistEnabled = pkgi18n.MsgOptInWatchlistEnabled
	msgOptInDisabled         = pkgi18n.MsgOptInDisabled
	msgOptInInvalidChoice    = pkgi18n.MsgOptInInvalidChoice

	// Memory control command responses.
	msgMemoryEmpty          = pkgi18n.MsgMemoryEmpty
	msgMemoryForgetInvalid  = pkgi18n.MsgMemoryForgetInvalid
	msgMemoryForgotConfirm  = pkgi18n.MsgMemoryForgotConfirm
	msgChatHistoryDeleted   = pkgi18n.MsgChatHistoryDeleted
	msgAllMemoryDeleted     = pkgi18n.MsgAllMemoryDeleted
)
