package pub.edholm.web

import org.springframework.context.annotation.Bean
import org.springframework.http.MediaType
import org.springframework.stereotype.Component
import org.springframework.web.reactive.function.server.router
import pub.edholm.web.handlers.AdminHandler
import pub.edholm.web.handlers.SwoshHandler

@Component
class Router(
  private val swoshHandler: SwoshHandler,
  private val adminHandler: AdminHandler
) {
  @Bean
  fun route() = router {
    accept(MediaType.TEXT_HTML).nest {
      "/admin".nest {
        GET("/", adminHandler::renderAdmin)
        GET("/{id}", adminHandler::renderSingle)
        POST("/{id}", adminHandler::update)
        DELETE("/{id}", adminHandler::delete)
      }

      GET("/", swoshHandler::renderIndex)
      GET("/{id}", swoshHandler::renderPreview)
      GET("/{id}/redir", swoshHandler::redirectToSwish)
      GET("/{id}/qr", swoshHandler::renderQRCode)
    }

    // API-routes
    (accept(MediaType.APPLICATION_JSON) and "/api").nest {
      POST("/create", swoshHandler::createSwosh)
    }
  }
}