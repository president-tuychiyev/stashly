package admin

import (
	"errors"

	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/requests"
	"github.com/president-tuychiyev/stashly/api/app/http/responses"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/resources"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type ClientController struct {
	clients *services.ClientService
	storage *services.StorageService
	audit   *services.AuditService
}

func NewClientController() *ClientController {
	return &ClientController{
		clients: services.NewClientService(),
		storage: services.NewStorageService(),
		audit:   services.NewAuditService(),
	}
}

// Index lists the clients.
func (r *ClientController) Index(ctx http.Context) http.Response {
	pagination := responses.ReadPagination(ctx)

	query := services.Scope(ctx).ApplyToClients(facades.Orm().Query().Model(&models.Client{}).With("Owner"))
	if search := ctx.Request().Query("search"); search != "" {
		pattern := "%" + services.EscapeLike(search) + "%"
		query = query.Where("(name ILIKE ? OR username ILIKE ?)", pattern, pattern)
	}
	if status := ctx.Request().Query("status"); status != "" {
		query = query.Where("status", status)
	}
	query = services.ApplySort(query, ctx.Request().Query("sort"), []string{"created_at", "name", "username", "id"}, "created_at")

	var clients []models.Client
	var total int64
	if err := query.Paginate(pagination.Page, pagination.PerPage, &clients, &total); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	out := make([]map[string]any, 0, len(clients))
	for index := range clients {
		used, count := r.usage(clients[index].ID)
		out = append(out, resources.Client(&clients[index], used, count))
	}

	return responses.Paginated(ctx, out, pagination, total)
}

// Store creates a client and returns its plain password once.
func (r *ClientController) Store(ctx http.Context) http.Response {
	var request requests.CreateClientRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	// The check mirrors the database index exactly: clients_username_active_unique
	// is a partial unique index over the live rows only, so a soft deleted
	// client no longer holds its username and the name can be reused. Anything
	// the index would refuse is caught here first and answered with a 422
	// instead of a raw 500 from the driver.
	taken, err := facades.Orm().Query().Model(&models.Client{}).
		Where("username", request.Username).Exists()
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}
	if taken {
		return responses.InvalidField(ctx, "username", "username is already taken")
	}

	owner, ownerErr := resolveOwner(ctx, request.OwnerID)
	if ownerErr != nil {
		return responses.InvalidField(ctx, "owner_id", ownerErr.Error())
	}

	input := services.ClientInput{
		Name:        &request.Name,
		Username:    &request.Username,
		QuotaBytes:  request.QuotaBytes,
		MaxFileSize: request.MaxFileSize,
		OwnerID:     &owner,
	}
	if request.Password != "" {
		input.Password = &request.Password
	}
	if request.Status != "" {
		input.Status = &request.Status
	}
	if request.AllowedMimes != nil {
		input.AllowedMimes = &request.AllowedMimes
	}

	client, password, err := r.clients.Create(input, AuthUserID(ctx))
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	id := client.ID
	r.audit.Log(Actor(ctx), "client.create", "client", &id,
		map[string]any{"username": request.Username, "owner_id": owner}, ctx.Request().Ip())

	r.loadOwner(client)

	return ctx.Response().Status(http.StatusCreated).Json(http.Json{
		"data":     resources.Client(client, 0, 0),
		"password": password,
	})
}

// Show returns one client.
func (r *ClientController) Show(ctx http.Context) http.Response {
	client, found := findClient(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	r.loadOwner(client)
	used, count := r.usage(client.ID)

	return responses.Data(ctx, http.StatusOK, resources.Client(client, used, count))
}

// Update changes the writable fields of a client.
func (r *ClientController) Update(ctx http.Context) http.Response {
	client, found := findClient(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	var request requests.UpdateClientRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	if request.Username != nil && *request.Username != "" {
		taken, err := facades.Orm().Query().Model(&models.Client{}).
			Where("username", *request.Username).Where("id != ?", client.ID).Exists()
		if err != nil {
			return responses.Error(ctx, http.StatusInternalServerError, err.Error())
		}
		if taken {
			return responses.InvalidField(ctx, "username", "username is already taken")
		}
	}

	input := services.ClientInput{
		Name:         request.Name,
		Username:     request.Username,
		Status:       request.Status,
		QuotaBytes:   request.QuotaBytes,
		AllowedMimes: request.AllowedMimes,
		MaxFileSize:  request.MaxFileSize,
	}

	// Only a super admin may hand a client to somebody else; for an admin the
	// field is ignored and ownership stays where it is.
	if services.Scope(ctx).IsSuperAdmin && request.OwnerID != nil {
		owner, ownerErr := resolveOwner(ctx, request.OwnerID)
		if ownerErr != nil {
			return responses.InvalidField(ctx, "owner_id", ownerErr.Error())
		}
		input.OwnerID = &owner
	}

	// A nullable field sent explicitly as null clears the stored value, while
	// a field that is simply absent leaves it untouched.
	body := ctx.Request().All()
	if value, sent := body["quota_bytes"]; sent && value == nil {
		input.ClearQuotaBytes = true
	}
	if value, sent := body["max_file_size"]; sent && value == nil {
		input.ClearMaxFileSize = true
	}

	if err := r.clients.Update(client, input, AuthUserID(ctx)); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	id := client.ID
	r.audit.Log(Actor(ctx), "client.update", "client", &id, map[string]any{}, ctx.Request().Ip())

	r.loadOwner(client)
	used, count := r.usage(client.ID)

	return responses.Data(ctx, http.StatusOK, resources.Client(client, used, count))
}

// ResetPassword issues a new password for a client.
func (r *ClientController) ResetPassword(ctx http.Context) http.Response {
	client, found := findClient(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	var request requests.ResetPasswordRequest
	if errs, err := ctx.Request().ValidateRequest(&request); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	} else if errs != nil {
		return responses.FromValidationErrors(ctx, errs)
	}

	var supplied *string
	if request.Password != "" {
		supplied = &request.Password
	}

	password, err := r.clients.ResetPassword(client, supplied, AuthUserID(ctx))
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	id := client.ID
	r.audit.Log(Actor(ctx), "client.reset_password", "client", &id, map[string]any{}, ctx.Request().Ip())

	r.loadOwner(client)
	used, count := r.usage(client.ID)

	return ctx.Response().Success().Json(http.Json{
		"data":     resources.Client(client, used, count),
		"password": password,
	})
}

// Destroy removes a client. With ?purge=true its files are deleted too.
func (r *ClientController) Destroy(ctx http.Context) http.Response {
	client, found := findClient(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	purge := ctx.Request().QueryBool("purge", false)
	if purge {
		var files []models.File
		if err := facades.Orm().Query().Model(&models.File{}).Where("client_id", client.ID).Get(&files); err != nil {
			return responses.Error(ctx, http.StatusInternalServerError, err.Error())
		}
		for index := range files {
			if err := r.storage.Delete(&files[index]); err != nil {
				facades.Log().Warning("failed to purge client file: " + err.Error())
			}
		}
	}

	if _, err := facades.Orm().Query().Delete(client); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	r.clients.ForgetCache(client)

	id := client.ID
	r.audit.Log(Actor(ctx), "client.delete", "client", &id, map[string]any{"purge": purge}, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

// Devices lists the devices that authenticated as a client.
func (r *ClientController) Devices(ctx http.Context) http.Response {
	client, found := findClient(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	pagination := responses.ReadPagination(ctx)

	var devices []models.Device
	var total int64
	err := facades.Orm().Query().Model(&models.Device{}).
		Where("client_id", client.ID).
		OrderByDesc("last_seen_at").
		Paginate(pagination.Page, pagination.PerPage, &devices, &total)
	if err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	return responses.Paginated(ctx, resources.Devices(devices), pagination, total)
}

// DestroyDevice removes one device of a client.
func (r *ClientController) DestroyDevice(ctx http.Context) http.Response {
	client, found := findClient(ctx)
	if !found {
		return responses.NotFound(ctx)
	}

	deviceID := ctx.Request().RouteInt("device_id")

	var device models.Device
	if err := facades.Orm().Query().Where("id", deviceID).Where("client_id", client.ID).First(&device); err != nil || device.ID == 0 {
		return responses.NotFound(ctx)
	}

	if _, err := facades.Orm().Query().Delete(&device); err != nil {
		return responses.Error(ctx, http.StatusInternalServerError, err.Error())
	}

	// Revoking a device cuts off whoever was using it, which is exactly the
	// kind of change the audit trail exists for.
	clientID := client.ID
	details := map[string]any{
		"device_id": device.ID,
		"uid":       device.UID,
		"platform":  device.Platform,
	}
	r.audit.LogFor(Actor(ctx), "client.device_delete", "client", &clientID, &clientID,
		details, ctx.Request().Ip())

	return responses.NoContent(ctx)
}

// usage returns the bytes used and the file count of a client.
func (r *ClientController) usage(clientID uint) (int64, int64) {
	used, err := r.storage.UsedBytes(clientID)
	if err != nil {
		used = 0
	}

	count, err := facades.Orm().Query().Model(&models.File{}).Where("client_id", clientID).Count()
	if err != nil {
		count = 0
	}

	return used, count
}

// findClient loads the client of the route, but only when the caller owns it.
// A client of another admin is reported as missing, never as forbidden, so the
// ids of other tenants stay unguessable.
func findClient(ctx http.Context) (*models.Client, bool) {
	id := ctx.Request().RouteInt("id")
	if id <= 0 {
		return nil, false
	}

	var client models.Client
	if err := facades.Orm().Query().Where("id", id).First(&client); err != nil || client.ID == 0 {
		return nil, false
	}

	if !services.Scope(ctx).OwnsClient(&client) {
		return nil, false
	}

	return &client, true
}

// resolveOwner decides who owns a client. An admin always owns what it
// touches; a super admin may name any active user and defaults to itself.
func resolveOwner(ctx http.Context, requested *uint) (uint, error) {
	scope := services.Scope(ctx)
	if !scope.IsSuperAdmin || requested == nil || *requested == 0 {
		return scope.UserID, nil
	}

	var owner models.User
	if err := facades.Orm().Query().Where("id", *requested).First(&owner); err != nil || owner.ID == 0 {
		return 0, errors.New("owner not found")
	}
	if owner.Status != models.UserStatusActive {
		return 0, errors.New("the owner has to be an active user")
	}

	return owner.ID, nil
}

// loadOwner fills in the owner relation for the response.
func (r *ClientController) loadOwner(client *models.Client) {
	if client == nil || client.OwnerID == nil || client.Owner != nil {
		return
	}

	var owner models.User
	if err := facades.Orm().Query().Where("id", *client.OwnerID).First(&owner); err == nil && owner.ID != 0 {
		client.Owner = &owner
	}
}
