package pub.edholm.services

import io.micrometer.core.instrument.MeterRegistry
import org.slf4j.LoggerFactory
import org.springframework.scheduling.annotation.Scheduled
import org.springframework.stereotype.Service
import pub.edholm.db.SwoshRepository
import java.time.Instant

@Service
class ExpiryService(
  private val swoshRepo: SwoshRepository,
  meterRegistry: MeterRegistry
) {

  private val counter = meterRegistry.counter("expired.links")
  private val logger = LoggerFactory.getLogger(ExpiryService::class.java)

  @Scheduled(fixedRateString = "\${swosh.expire.rate}", initialDelay = 1337)
  fun expireOldLinks() {
    val now = Instant.now()
    val expired = swoshRepo.findByExpiresOnBefore(now)
    expired.count().subscribe { c ->
      if (c > 0) {
        logger.info("Expiring {} links before {}", c, now)
      }
      counter.increment(c.toDouble())
    }
    swoshRepo.deleteAll(expired).subscribe()
  }
}