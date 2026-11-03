package seed

// Sample content. Every sentence here was written for this seed: harmless,
// everyday and obviously not client data. Swahili lines have an English
// gloss in the comment.

type sampleItem struct {
	externalID string
	content    map[string]any
	answer     map[string]any // the correct answer, used for gold items and qualification questions
	gold       bool           // also a gold item in its sample project
}

var classificationLabels = []string{"Billing", "Network", "Account", "Other"}

var classificationItems = []sampleItem{
	{"sample-cls-001", map[string]any{"text": "Nimeongeza salio lakini bado halijaonekana."}, // I topped up but it has not shown yet.
		map[string]any{"labels": []string{"Billing"}}, true},
	{"sample-cls-002", map[string]any{"text": "Mtandao uko chini sana leo asubuhi."}, // The network is very weak this morning.
		map[string]any{"labels": []string{"Network"}}, true},
	{"sample-cls-003", map[string]any{"text": "Nimesahau nenosiri la akaunti yangu."}, map[string]any{"labels": []string{"Account"}}, false}, // I forgot my account password.
	{"sample-cls-004", map[string]any{"text": "Asante kwa huduma nzuri jana."}, map[string]any{"labels": []string{"Other"}}, false},          // Thank you for the good service yesterday.
	{"sample-cls-005", map[string]any{"text": "My data bundle finished faster than I expected."}, map[string]any{"labels": []string{"Billing"}}, false},
	{"sample-cls-006", map[string]any{"text": "Calls keep dropping when I am in town."}, map[string]any{"labels": []string{"Network"}}, false},
	{"sample-cls-007", map[string]any{"text": "Please help me change the name on my account."}, map[string]any{"labels": []string{"Account"}}, false},
	{"sample-cls-008", map[string]any{"text": "Duka lenu linafunguliwa saa ngapi?"}, map[string]any{"labels": []string{"Other"}}, false}, // What time does your shop open?
}

var pairwiseRubric = []string{"accuracy", "helpfulness", "safety", "language_quality"}

var pairwiseItems = []sampleItem{
	{"sample-pw-001", map[string]any{
		"prompt":     "How often should I water tomato seedlings?",
		"response_a": "Water them lightly every morning so the soil stays moist but not soaked. Check the top of the soil with a finger first.",
		"response_b": "Tomatoes need a lot of water, so flood the tray twice a day.",
	}, map[string]any{"preference": "a"}, true},
	{"sample-pw-002", map[string]any{
		"prompt":     "Explain what a budget is in one sentence.",
		"response_a": "A budget is a plan for how you will spend and save the money you expect to receive.",
		"response_b": "A budget is when you write numbers.",
	}, map[string]any{"preference": "a"}, false},
	{"sample-pw-003", map[string]any{
		"prompt":     "Nipe vidokezo viwili vya kupika wali mzuri.",                                // Give me two tips for cooking good rice.
		"response_a": "Osha mchele kwanza, kisha tumia maji kiasi sahihi na upike kwa moto mdogo.", // Wash the rice first, then use the right amount of water and cook on low heat.
		"response_b": "Pika kwa moto mkali sana bila kufunika sufuria.",                            // Cook on very high heat without covering the pot.
	}, map[string]any{"preference": "a"}, false},
	{"sample-pw-004", map[string]any{
		"prompt":     "What should I pack for a one-day hike?",
		"response_a": "Water, a snack, a hat and a light jacket.",
		"response_b": "Water, food, sun protection, a warm layer, a charged phone, and tell someone your route.",
	}, map[string]any{"preference": "b"}, false},
}

var translationItems = []sampleItem{
	{"sample-tr-001", map[string]any{
		"source": "The market opens at eight in the morning.", "translation": "Soko linafunguliwa saa mbili asubuhi.",
		"source_lang": "en", "target_lang": "sw",
	}, map[string]any{"adequacy": 5, "fluency": 5}, true},
	{"sample-tr-002", map[string]any{
		"source": "Please bring your receipt when you return.", "translation": "Tafadhali leta risiti yako ukirudi.",
		"source_lang": "en", "target_lang": "sw",
	}, map[string]any{"adequacy": 5, "fluency": 5}, false},
	{"sample-tr-003", map[string]any{
		"source": "The bus leaves when it is full.", "translation": "Basi inaondoka wakati imejaa sana.",
		"source_lang": "en", "target_lang": "sw",
	}, map[string]any{"adequacy": 4, "fluency": 3}, false},
	{"sample-tr-004", map[string]any{
		"source": "Our office is closed on public holidays.", "translation": "Ofisi yetu imefungwa siku za sikukuu.",
		"source_lang": "en", "target_lang": "sw",
	}, map[string]any{"adequacy": 5, "fluency": 5}, false},
}
