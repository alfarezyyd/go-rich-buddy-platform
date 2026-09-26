package validator

type Service interface {
	ValidateStruct(targetValidatorStruct interface{}) error
	ValidateVar(targetValidatorStruct interface{}, validatorTags string) error
	ParseValidationError(validationError error, dtoStruct interface{})
}
