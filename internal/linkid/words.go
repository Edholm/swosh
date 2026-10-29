package linkid

// The word lists are kept sorted, lowercase and free of words that
// could make a link awkward to send to a friend.

// adjectives start an id.
var adjectives = []string{
	"able", "agile", "airy", "amber", "ample", "aqua", "azure", "balmy", "bold", "bouncy",
	"brave", "breezy", "bright", "brisk", "bubbly", "busy", "calm", "candid", "carefree",
	"charming", "chatty", "cheeky", "cheery", "chipper", "chirpy", "civil", "classy",
	"clean", "clear", "clever", "cloudy", "comfy", "cool", "coral", "cosmic", "cozy",
	"crafty", "creamy", "crimson", "crisp", "cuddly", "curious", "curly", "cute", "dainty",
	"dandy", "daring", "dashing", "dazzling", "deft", "dewy", "dreamy", "dusky", "eager",
	"early", "earthy", "easy", "elegant", "epic", "exotic", "fair", "fancy", "fast",
	"festive", "fine", "fizzy", "fluffy", "fond", "frank", "free", "fresh", "frosty",
	"fruity", "funky", "funny", "fuzzy", "gentle", "giant", "gifted", "glad", "gleeful",
	"glossy", "golden", "good", "grand", "great", "green", "groovy", "handy", "happy",
	"hardy", "hearty", "helpful", "heroic", "honest", "hopeful", "humble", "icy", "ideal",
	"indigo", "ivory", "jade", "jazzy", "jolly", "jovial", "joyful", "jumbo", "keen", "kind",
	"kindly", "lavish", "lilac", "lime", "lively", "lofty", "lovely", "loyal", "lucky",
	"lunar", "lush", "magic", "majestic", "mellow", "merry", "mighty", "mild", "minty",
	"misty", "modern", "modest", "navy", "neat", "nifty", "nimble", "noble", "orange",
	"peachy", "peppy", "perky", "pink", "plucky", "plush", "polite", "posh", "proud",
	"purple", "quick", "quiet", "quirky", "radiant", "rapid", "rare", "ready", "red",
	"regal", "rosy", "round", "royal", "ruby", "rugged", "rustic", "sandy", "savvy",
	"scarlet", "shiny", "silky", "silly", "silver", "simple", "sleek", "sleepy", "slick",
	"smart", "smiley", "smooth", "snappy", "snazzy", "snowy", "snug", "soft", "solar",
	"solid", "sparkly", "speedy", "spicy", "spiffy", "spry", "starry", "steady", "stellar",
	"sturdy", "sunny", "super", "sweet", "swift", "tall", "tangy", "tasty", "teal", "tender",
	"tidy", "tiny", "toasty", "trusty", "upbeat", "vast", "velvet", "violet", "vivid",
	"warm", "wavy", "wild", "windy", "wise", "witty", "woolly", "worthy", "yellow", "young",
	"yummy", "zany", "zesty", "zippy",
}

// nouns are mostly animals, food and things in nature.
var nouns = []string{
	"acorn", "alpaca", "anchor", "apple", "apricot", "avocado", "badger", "bagel", "banjo",
	"basil", "beacon", "bear", "beaver", "bee", "beetle", "berry", "biscuit", "bison",
	"blossom", "breeze", "bubble", "bunny", "button", "cactus", "camel", "canary", "candle",
	"canoe", "carrot", "castle", "cello", "cheetah", "cherry", "chick", "chipmunk", "clam",
	"cloud", "clover", "cocoa", "coconut", "comet", "condor", "cookie", "corgi", "cotton",
	"cougar", "crab", "crane", "crayon", "cricket", "crow", "cupcake", "daisy", "deer",
	"dingo", "dodo", "dolphin", "dove", "dragon", "duck", "eagle", "elk", "ember", "emu",
	"falcon", "feather", "fern", "ferret", "fig", "finch", "flute", "fox", "frog", "garden",
	"gazelle", "gecko", "gerbil", "ginger", "giraffe", "glacier", "goat", "goose", "gopher",
	"guitar", "hamster", "harbor", "hare", "harp", "hawk", "hazel", "hedgehog", "heron",
	"hippo", "honey", "horse", "husky", "ibis", "iguana", "impala", "island", "jaguar",
	"jelly", "kayak", "kettle", "kite", "kitten", "kiwi", "koala", "lamb", "lantern", "lark",
	"lemon", "lemur", "leopard", "lily", "lion", "llama", "lobster", "lotus", "lynx",
	"magpie", "mammoth", "manatee", "mango", "maple", "marble", "meadow", "meerkat", "melon",
	"meteor", "mitten", "moon", "moose", "mouse", "muffin", "narwhal", "nectar", "newt",
	"noodle", "nugget", "oak", "oasis", "ocelot", "octopus", "olive", "orca", "orchid",
	"osprey", "ostrich", "otter", "owl", "oyster", "pancake", "panda", "papaya", "parrot",
	"peacock", "pebble", "pelican", "penguin", "pepper", "piano", "pickle", "piglet", "pine",
	"planet", "plum", "pony", "poodle", "pretzel", "puffin", "puma", "pumpkin", "quail",
	"quartz", "rabbit", "raccoon", "radish", "rainbow", "raisin", "raven", "river", "robin",
	"rocket", "salmon", "scone", "seal", "shell", "shrimp", "sloth", "snail", "sparrow",
	"sprout", "squid", "star", "stork", "sunset", "swan", "tapir", "teapot", "thistle",
	"tiger", "toad", "tomato", "toucan", "trout", "tulip", "tuna", "turnip", "turtle",
	"ukulele", "violin", "waffle", "walnut", "walrus", "whale", "willow", "wolf", "wombat",
	"wren", "yak", "yogurt", "zebra",
}

// verbs are in the present tense, so that an id reads as a sentence.
var verbs = []string{
	"bakes", "beams", "bikes", "blinks", "blooms", "bobs", "bops", "bounces", "bows",
	"builds", "camps", "chats", "cheers", "chirps", "chuckles", "claps", "climbs", "cooks",
	"counts", "crawls", "cruises", "cycles", "dances", "dashes", "digs", "dives", "doodles",
	"dozes", "dreams", "drifts", "drives", "drums", "explores", "fishes", "flaps", "flies",
	"floats", "flutters", "frolics", "gallops", "gazes", "giggles", "glides", "glimmers",
	"glows", "grins", "grooves", "grows", "hikes", "hops", "hovers", "hugs", "hums",
	"invents", "jives", "jogs", "juggles", "jumps", "kayaks", "knits", "laughs", "leaps",
	"learns", "lingers", "listens", "lounges", "marches", "mingles", "munches", "naps",
	"nestles", "nibbles", "nods", "nuzzles", "paddles", "paints", "peeks", "perches",
	"plays", "ponders", "pounces", "prances", "purrs", "races", "rafts", "rambles", "reads",
	"relaxes", "rests", "rides", "roams", "rolls", "rows", "runs", "sails", "scoots",
	"sculpts", "scurries", "sews", "shimmers", "shines", "sings", "sips", "skates",
	"sketches", "skips", "skis", "sleds", "slides", "smiles", "snacks", "snoozes",
	"snuggles", "soars", "sparkles", "spins", "splashes", "sprints", "strolls", "strums",
	"studies", "surfs", "sways", "swims", "swings", "swoops", "tangos", "teaches", "thinks",
	"tinkers", "tiptoes", "travels", "trots", "tumbles", "twinkles", "twirls", "waddles",
	"wades", "waits", "waltzes", "wanders", "waves", "whistles", "wiggles", "winks",
	"wobbles", "wonders", "writes", "yawns", "yodels", "zigzags", "zooms",
}
