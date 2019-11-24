package pub.edholm

import org.springframework.context.annotation.Bean
import org.springframework.security.config.annotation.method.configuration.EnableReactiveMethodSecurity
import org.springframework.security.config.annotation.web.reactive.EnableWebFluxSecurity
import org.springframework.security.config.web.server.ServerHttpSecurity
import org.springframework.security.core.GrantedAuthority
import org.springframework.security.web.server.SecurityWebFilterChain

@EnableWebFluxSecurity
@EnableReactiveMethodSecurity
class SecurityConfig {
  @Bean
  fun securityWebFilterChain(http: ServerHttpSecurity): SecurityWebFilterChain {
    return http.authorizeExchange()
      .pathMatchers("/admin/**")
      .hasRole("ADMIN")
      .anyExchange()
      .permitAll()
      .and()
      .csrf().disable()
      .headers()
      // https://scotthelme.co.uk/content-security-policy-an-introduction/
      .contentSecurityPolicy("default-src 'self'; script-src https://use.fontawesome.com:443 https://code.jquery.com:443 'self' 'unsafe-inline'; style-src https://maxcdn.bootstrapcdn.com:443 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'")
      .and().and()
      .httpBasic()
      .and()
      .formLogin()
      .and()
      .build()
  }

  fun isAdmin(authorities: Collection<GrantedAuthority>): Boolean = authorities.any { it.authority == "ROLE_ADMIN" }
}