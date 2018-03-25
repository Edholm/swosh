package pub.edholm.web.handlers

import org.springframework.data.mongodb.repository.ReactiveMongoRepository
import org.springframework.stereotype.Repository
import reactor.core.publisher.Flux
import java.time.Instant

@Repository
interface SwoshRepository : ReactiveMongoRepository<Swosh, String> {
  fun findByExpiresOnBefore(now: Instant): Flux<Swosh>
}