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
        POST("/{id}", adminHandler::update)
        DELETE("/{id}", adminHandler::delete)
      }

      GET("/", swoshHandler::renderIndex)
      GET("/{id}", swoshHandler::renderPreview)
      GET("/{id}/redir", swoshHandler::redirectToSwish)
    }

    // API-routes
    (accept(MediaType.APPLICATION_JSON_UTF8) and "/api").nest {
      POST("/create", swoshHandler::createSwosh)
    }
  }
}