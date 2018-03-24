package pub.edholm.authentication

import org.springframework.security.core.userdetails.ReactiveUserDetailsService
import org.springframework.security.core.userdetails.UserDetails
import org.springframework.stereotype.Service
import reactor.core.publisher.Mono

@Service
class SwoshUserDetailsService(private val userRepository: UserRepository) : ReactiveUserDetailsService {
  override fun findByUsername(username: String): Mono<UserDetails> {
    return userRepository.findByUsername(username)
      .cast(UserDetails::class.java)
  }
}