// Sample documents in the shape Spring Data wrote them, for trying
// mongo-migrate locally (mongo 4 shell syntax):
//
//   docker compose -f docker-compose.dev.yml --profile mongo up -d mongodb
//   docker compose -f docker-compose.dev.yml exec -T mongodb mongo swosh /seed/seed.js
//
// Expected import: 4 links (old123 is expired and skipped) and 2 users.
const day = 24 * 60 * 60 * 1000;
const now = Date.now();

db.swosh.drop();
db.swosh.insertMany([
  {
    _id: "abc123",
    payee: "070-000 00 00",
    amount: NumberInt(100),
    description: "Pizza",
    expiresOn: new Date(now + 2 * day),
    createdAt: new Date(now),
    _class: "pub.edholm.db.Swosh",
  },
  {
    _id: "nomsg1",
    payee: "073-000 00 00",
    amount: NumberInt(50),
    expiresOn: new Date(now + day),
    createdAt: new Date(now),
    _class: "pub.edholm.db.Swosh",
  },
  {
    _id: "never1",
    payee: "1230000000",
    amount: NumberInt(1337),
    description: "Forever",
    createdAt: new Date(now - 400 * day),
    _class: "pub.edholm.db.Swosh",
  },
  {
    // Saved through the admin form, which stored an empty description.
    _id: "blank1",
    payee: "076-000 00 00",
    amount: NumberInt(0),
    description: "",
    createdAt: new Date(now - 10 * day),
    _class: "pub.edholm.db.Swosh",
  },
  {
    _id: "old123",
    payee: "070-111 11 11",
    amount: NumberInt(10),
    description: "Expired",
    expiresOn: new Date(now - day),
    createdAt: new Date(now - 3 * day),
    _class: "pub.edholm.db.Swosh",
  },
]);

const authority = (role) => ({
  role: role,
  _class: "org.springframework.security.core.authority.SimpleGrantedAuthority",
});

db.user.drop();
db.user.insertMany([
  {
    _id: BinData(3, "AAECAwQFBgcICQoLDA0ODw=="),
    username: "admin",
    // bcrypt of "admin", the Kotlin service's default SWOSH_ADMIN_PWD.
    password: "{bcrypt}$2a$10$0MfTyXDYOZTLPDoT7hqrr.xH1qI8A8s/DmBMp1R94iNznJsP0r.6y",
    isEnabled: true,
    authorities: [authority("ROLE_USER"), authority("ROLE_ADMIN"), authority("DELETE"), authority("UPDATE")],
    _class: "pub.edholm.db.User",
  },
  {
    _id: BinData(3, "EBESExQVFhcYGRobHB0eHw=="),
    username: "viewer",
    password: "{bcrypt}$2a$10$0MfTyXDYOZTLPDoT7hqrr.xH1qI8A8s/DmBMp1R94iNznJsP0r.6y",
    authorities: [authority("USER")],
    _class: "pub.edholm.db.User",
  },
]);

db.sessions.drop();
db.sessions.insertOne({ _id: "ignored-session", principal: "admin" });
