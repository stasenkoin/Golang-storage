package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"filestorage/internal/core/config"
	"filestorage/internal/features/auth"
	"filestorage/internal/features/files"
	"filestorage/internal/features/permissions"
	"filestorage/internal/features/shares"
	"filestorage/internal/features/storages"
	appmw "filestorage/internal/transport/http/middleware"
	"filestorage/internal/transport/http/response"
)

func NewRouter(pool *pgxpool.Pool, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	authRepo := auth.NewRepository(pool)
	authService := auth.NewService(authRepo, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authHandler := auth.NewHandler(authService)

	storagesRepo := storages.NewRepository(pool)
	storagesService := storages.NewService(storagesRepo, cfg.FileStoragePath)
	storagesHandler := storages.NewHandler(storagesService)

	filesRepo := files.NewRepository(pool)
	filesService := files.NewService(filesRepo, storagesRepo, cfg.FileStoragePath)
	filesHandler := files.NewHandler(filesService)

	permsRepo := permissions.NewRepository(pool)
	permsService := permissions.NewService(permsRepo, storagesRepo)
	permsHandler := permissions.NewHandler(permsService)

	sharesRepo := shares.NewRepository(pool)
	sharesService := shares.NewService(sharesRepo, filesRepo, storagesRepo)
	sharesHandler := shares.NewHandler(sharesService)

	r.Route("/api", func(api chi.Router) {
		api.Get("/health", healthHandler)

		api.Route("/auth", func(ar chi.Router) {
			ar.Post("/register", authHandler.Register)
			ar.Post("/login", authHandler.Login)
			ar.Post("/refresh", authHandler.Refresh)
			ar.Post("/logout", authHandler.Logout)
			ar.Group(func(pr chi.Router) {
				pr.Use(appmw.Auth(cfg.JWTSecret))
				pr.Get("/me", authHandler.Me)
			})
		})

		api.Group(func(pr chi.Router) {
			pr.Use(appmw.Auth(cfg.JWTSecret))

			pr.Post("/storages", storagesHandler.Create)
			pr.Get("/storages/my", storagesHandler.ListMy)
			pr.Get("/storages/global", storagesHandler.ListGlobal)
			pr.Get("/storages/shared", storagesHandler.ListShared)
			pr.Get("/storages/{id}", storagesHandler.Get)
			pr.Patch("/storages/{id}", storagesHandler.Update)
			pr.Delete("/storages/{id}", storagesHandler.Delete)

			pr.Post("/storages/{id}/files", filesHandler.Upload)
			pr.Get("/storages/{id}/files", filesHandler.List)
			pr.Get("/files/{id}/download", filesHandler.Download)
			pr.Delete("/files/{id}", filesHandler.Delete)

			pr.Post("/storages/{id}/permissions", permsHandler.Grant)
			pr.Get("/storages/{id}/permissions", permsHandler.List)
			pr.Patch("/storages/{id}/permissions/{permissionId}", permsHandler.Update)
			pr.Delete("/storages/{id}/permissions/{permissionId}", permsHandler.Delete)

			pr.Post("/files/{id}/share", sharesHandler.Create)
			pr.Get("/files/{id}/share-links", sharesHandler.ListLinks)
			pr.Delete("/share-links/{id}", sharesHandler.Revoke)
			pr.Get("/shared/{token}", sharesHandler.Open)
			pr.Get("/shared-files", sharesHandler.ListSharedFiles)
		})
	})

	fileServer := http.FileServer(http.Dir("public"))
	r.Handle("/*", fileServer)

	return r
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
