package validation

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/robfig/cron/v3"
	"go-work/internal/http/constants"
	"go-work/internal/model"
)

func RegisterJobValidation(validate *validator.Validate, storage model.JobStorage) error {
	err := validate.RegisterValidationCtx("uniqueName", func(ctx context.Context, fl validator.FieldLevel) bool {
		timeoutCtx, cancel := context.WithTimeout(ctx, constants.StorageOperationTimeout)
		defer cancel()
		_, err := storage.GetJobByName(timeoutCtx, fl.Field().String())
		if err != nil && errors.Is(err, model.ErrorNotFound) {
			return true
		}
		return false
	})
	if err != nil {
		return fmt.Errorf("failed registering the \"uniqueName\" validation tag: %w", err)
	}

	err = validate.RegisterValidation("crontabString", func(fl validator.FieldLevel) bool {
		_, err := cron.ParseStandard(fl.Field().String())
		return err == nil
	})
	if err != nil {
		err = fmt.Errorf("failed registering the \"crontabString\" validation tag: %w", err)
	}
	return err
}
