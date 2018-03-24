package pub.edholm

import org.springframework.context.annotation.Bean
import org.springframework.security.config.annotation.web.reactive.EnableWebFluxSecurity
import org.springframework.security.config.web.server.ServerHttpSecurity
import org.springframework.security.web.server.SecurityWebFilterChain

@EnableWebFluxSecurity
class SecurityConfig {
  @Bean
  fun securityWebFilterChain(http: ServerHttpSecurity): SecurityWebFilterChain {
    return http.authorizeExchange()
      .pathMatchers("/actuator/**")
      .hasRole("ADMIN")
      .anyExchange()
      .permitAll()
      .and()
      .csrf().disable()
      .httpBasic()
      .and()
      .formLogin()
      .and()
      .build()
  }
}