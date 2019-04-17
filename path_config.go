package main

import (
	"context"
	"errors"

	"github.com/asaskevich/govalidator"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/sdk/framework"
)

const (
	configStoragePath = "config"
)

// vmwareConfig contains values to configure vmware clients
type vmwareConfig struct {
	AuthenticationURL string `json:"authentication_url"`
	APIURL            string `json:"api_url"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	Region            string `json:"region"`
}

func pathConfig(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "config",
		Fields: map[string]*framework.FieldSchema{
			"authentication_url": {
				Type:        framework.TypeString,
				Description: `The URL which is used for authentication and token generation.`,
			},
			"api_url": {
				Type:        framework.TypeString,
				Description: `The URL which is used for communication with vRO API`,
			},
			"username": {
				Type:        framework.TypeString,
				Description: `Subscription username`,
			},
			"password": {
				Type:        framework.TypeString,
				Description: `Subscription password`,
			},
			"region": {
				Type:        framework.TypeString,
				Description: `Subscription region`,
			},
		},
		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.ReadOperation:   b.pathConfigRead,
			logical.CreateOperation: b.pathConfigWrite,
			logical.UpdateOperation: b.pathConfigWrite,
			logical.DeleteOperation: b.pathConfigDelete,
		},
		ExistenceCheck:  b.pathConfigExistenceCheck,
		HelpSynopsis:    confHelpSyn,
		HelpDescription: confHelpDesc,
	}
}

func (b *backend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	var merr *multierror.Error

	config, err := b.getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if config == nil {
		if req.Operation == logical.UpdateOperation {
			return nil, errors.New("config not found during update operation")
		}
		config = new(vmwareConfig)
	}

	if authURL, ok := data.GetOk("authentication_url"); !ok {
		merr = multierror.Append(merr, errors.New("Authentication url is required"))
	} else if !govalidator.IsURL(authURL.(string)) {
		merr = multierror.Append(merr, errors.New("Given authentication url is not valid url"))
	} else {
		config.AuthenticationURL = authURL.(string)
	}

	if apiURL, ok := data.GetOk("api_url"); !ok {
		merr = multierror.Append(merr, errors.New("API url is required"))
	} else if !govalidator.IsURL(apiURL.(string)) {
		merr = multierror.Append(merr, errors.New("Given API url is not valid url"))
	} else {
		config.APIURL = apiURL.(string)
	}

	if username, ok := data.GetOk("username"); !ok {
		merr = multierror.Append(merr, errors.New("Username is required"))
	} else {
		config.Username = username.(string)
	}

	if password, ok := data.GetOk("password"); !ok {
		merr = multierror.Append(merr, errors.New("Password is required"))
	} else {
		config.Password = password.(string)
	}

	if region, ok := data.GetOk("region"); !ok {
		merr = multierror.Append(merr, errors.New("Region is required"))
	} else {
		config.Region = region.(string)
	}

	if merr.ErrorOrNil() != nil {
		return logical.ErrorResponse(merr.Error()), nil
	}

	err = b.saveConfig(ctx, config, req.Storage)

	return nil, err
}

func (b *backend) pathConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := b.getConfig(ctx, req.Storage)

	if err != nil {
		return nil, err
	}

	if config == nil {
		config = new(vmwareConfig)
	}

	resp := &logical.Response{
		Data: map[string]interface{}{
			"authentication_url": config.AuthenticationURL,
			"api_url":            config.APIURL,
			"username":           config.Username,
			"region":             config.Region,
		},
	}
	return resp, nil
}

func (b *backend) pathConfigDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	err := req.Storage.Delete(ctx, configStoragePath)

	return nil, err
}

func (b *backend) pathConfigExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	config, err := b.getConfig(ctx, req.Storage)
	if err != nil {
		return false, err
	}

	return config != nil, err
}

func (b *backend) getConfig(ctx context.Context, s logical.Storage) (*vmwareConfig, error) {
	entry, err := s.Get(ctx, configStoragePath)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	config := new(vmwareConfig)
	if err := entry.DecodeJSON(config); err != nil {
		return nil, err
	}

	return config, nil
}

func (b *backend) saveConfig(ctx context.Context, config *vmwareConfig, s logical.Storage) error {
	entry, err := logical.StorageEntryJSON(configStoragePath, config)

	if err != nil {
		return err
	}

	err = s.Put(ctx, entry)
	if err != nil {
		return err
	}

	return nil
}

const confHelpSyn = `Configure the Azure Secret backend.`
const confHelpDesc = `
The Azure secret backend requires credentials for managing applications and
service principals. This endpoint is used to configure those credentials as
well as default values for the backend in general.
`
