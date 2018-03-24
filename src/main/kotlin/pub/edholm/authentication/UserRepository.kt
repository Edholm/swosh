package pub.edholm.authentication

import org.springframework.data.mongodb.repository.ReactiveMongoRepository
import org.springframework.stereotype.Repository
import reactor.core.publisher.Mono
import java.util.*

@Repository
interface UserRepository : ReactiveMongoRepository<User, UUID> {
  fun findByUsername(username: String): Mono<User>
}