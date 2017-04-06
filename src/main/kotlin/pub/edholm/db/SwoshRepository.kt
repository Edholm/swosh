package pub.edholm.db

import org.springframework.data.mongodb.repository.ReactiveMongoRepository
import org.springframework.stereotype.Repository

@Repository
interface SwoshRepository : ReactiveMongoRepository<Swosh, String>