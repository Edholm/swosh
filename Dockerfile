ARG IMAGE=openjdk:11-jre
FROM ${IMAGE}
VOLUME /tmp
COPY swosh.jar swosh.jar
ENTRYPOINT ["java", "-Duser.language=sv-SE", "-Dfile.encoding=UTF-8", "-Duser.timezon=UTC", "-Djava.security.egd=file:/dev/./urandom", "-jar","/swosh.jar"]
HEALTHCHECK --interval=10s --timeout=15s --retries=3 \
  CMD curl -f http://localhost:8080/actuator/health || exit 1
