// Package capability is the one place that records what Sehaty's tools need.
//
// Before this package the same knowledge lived in three: a hand-kept list of assessment
// questions in storage, a hand-written refusal inside each tool, and fifty lines of
// prose in the system prompt explaining why each question is asked. Three copies of one
// fact drift, and the drift is invisible — a tool gains an input and nobody adds the
// question, or a question is asked for a reason that stopped being true.
//
// Now the tool declares its need once, with the true reason attached, and the assessment,
// the refusals and the prompt are all derived from that declaration. A reason that is
// derived from a requirement is true by construction; a reason that is composed is only
// as true as whoever last edited the paragraph.
//
// This package imports nothing from the rest of Sehaty, deliberately: everything else may
// depend on it, so it may depend on nothing.
package capability

// Field is one thing Sehaty can know about a person.
//
// The string values are LOAD-BEARING. They are what `profile.answered` holds on disk, so
// changing one silently un-answers that question for every existing record and starts
// asking it again.
type Field string

const (
	FieldEquipment      Field = "equipment"
	FieldGoal           Field = "goal"
	FieldExperience     Field = "experience"
	FieldSchedule       Field = "training schedule"
	FieldInjuries       Field = "injuries or conditions"
	FieldHeight         Field = "height"
	FieldAge            Field = "age"
	FieldDietPreference Field = "diet preference"
	FieldAllergies      Field = "food allergies"
	FieldSex            Field = "sex"

	// FieldWeightLog is not a question. It is satisfied by a logged weight, so it never
	// appears in AskOrder — it is asked for by log_weight, not by the intake.
	FieldWeightLog Field = "weight_log"
)

// Question is a field that can be put to a person.
type Question struct {
	Field Field

	// Detectable says whether an absent value can be told from a given one.
	//
	// False for most of them, and that is the bug this flag exists to prevent: three
	// sessions a week is both the default and a perfectly normal answer, so inferring
	// "never asked" from the value would ask the one person who actually trains three
	// times a week about it every single day. For those fields, presence means the
	// question was put — a decline included.
	Detectable bool

	// Gate names the moment this must not be asked outside of. Empty means any time.
	//
	// Asked out of nowhere these read as data harvesting rather than as a health record
	// doing its job, and each has a moment that makes it obvious instead.
	Gate string

	// Because is the true consequence in the record, in the words the person gets if
	// they ask why. Never "to personalise your experience" — that is not true here and
	// they will smell it.
	Because string

	// Ask is the shape the question takes: Jakarta Indonesian, not textbook Indonesian.
	Ask string
}

// askOrder is the order to put the questions in, and the order is behaviour.
//
// Equipment is cheap to answer and age is not, so the list opens with something nobody
// minds and arrives at the personal end once there is a reason to be there. Sex is last
// because it usually settles itself in passing before it is ever reached.
var askOrder = []Question{
	{
		Field:   FieldEquipment,
		Because: "So a suggested session only uses things you actually have.",
		Ask:     "Biasa latihan pake apa — nge-gym, alat di rumah, atau bodyweight aja?",
	},
	{
		Field: FieldGoal,
		Because: "It decides what I watch in your numbers — a cut and a strength block " +
			"read the same log very differently.",
		Ask: "Latihannya lagi ngejar apa — nurunin lemak, nambah kuat, nambah otot, " +
			"atau jaga kondisi aja?",
	},
	{
		Field:   FieldExperience,
		Because: "So sessions are pitched where you are: not remedial, not reckless.",
		Ask:     "Udah berapa lama latihan? Baru mulai, atau udah lama?",
	},
	{
		Field: FieldSchedule,
		Because: "Every session I suggest is built on how often and how long you train. " +
			"It has been assuming three times a week for fifty minutes; I would rather know.",
		Ask: "Seminggu bisa latihan berapa kali, dan sekali latihan berapa lama?",
	},
	{
		Field: FieldInjuries,
		Because: "So I never suggest a movement that aggravates it, and so it is on the " +
			"record if a clinician ever reads this.",
		Ask: "Ada cedera lama atau bagian badan yang suka rewel? Lutut, bahu, pinggang.",
	},
	{
		Field:      FieldHeight,
		Detectable: true,
		Gate:       "only when a weight has just been logged",
		Because: "70 kg means something different at 165 cm and at 185. Height sits next " +
			"to your weight log so it reads properly.",
		Ask: "Tinggimu berapa? Angka berat lebih kebaca kalau ada tingginya.",
	},
	{
		Field:      FieldAge,
		Detectable: true,
		Because: "Recovery and pacing shift with age, so a session can suit yours — and a " +
			"doctor reading this record would expect it there.",
		Ask: "Umur berapa, kalau boleh tanya?",
	},
	{
		Field:   FieldDietPreference,
		Gate:    "only when food is being logged — vegetarian, non-vegetarian or vegan",
		Because: "So a suggestion is something you would actually eat.",
		Ask:     "Makannya ada aturan khusus? Vegetarian, vegan, atau makan semua?",
	},
	{
		Field: FieldAllergies,
		Gate:  "only when food is being logged",
		Because: "So it is flagged in your record, and I never suggest a food that would " +
			"hurt you.",
		Ask: "Sebelum catatan makannya makin panjang — ada alergi makanan?",
	},
	{
		Field:      FieldSex,
		Detectable: true,
		Gate: "prefer never asking — it usually surfaces on its own. Ask only when a " +
			"reference range or an exercise choice makes it concretely relevant",
		Because: "Strength references and some exercise choices differ. That is the whole use.",
		Ask: "Buat catatan aja — cowok atau cewek? Angka acuan sama beberapa pilihan " +
			"latihan emang beda.",
	},
}

// AskOrder returns the assessment questions in the order to put them.
func AskOrder() []Question {
	out := make([]Question, len(askOrder))
	copy(out, askOrder)
	return out
}

// Ask returns one question by field.
func Ask(f Field) (Question, bool) {
	for _, q := range askOrder {
		if q.Field == f {
			return q, true
		}
	}
	return Question{}, false
}

// Requirement is one field a tool needs.
type Requirement struct {
	Field Field

	// Because is the true reason, shown to the person. It says what this tool cannot do
	// without the field, not what the field is.
	Because string

	// Hard means refuse: a number would have to be invented. Soft means proceed AND name
	// the gap, which is a different failure from silence and keeps day-one friction low.
	Hard bool
}

// Capability is one tool and what it needs to work.
type Capability struct {
	Tool    string
	Needs   []Requirement
	Unlocks string
}

// known is the registry.
//
// Tools absent from this list need nothing and work from minute one: log_food, log_set,
// log_weight, log_cardio, find_foods, find_exercises, progress, get_profile. That is not
// an oversight — a health record that cannot be written to until a form is finished is a
// form, and only about one beginner in ten is still training at 52 weeks.
var known = []Capability{
	{
		Tool:    "estimate_energy",
		Unlocks: "an estimate of how much energy you use in a day",
		Needs: []Requirement{
			{Field: FieldAge, Hard: true,
				Because: "The equation is age-banded; without an age there is no coefficient to use."},
			{Field: FieldHeight, Hard: true,
				Because: "A weight on its own does not say what body it belongs to."},
			{Field: FieldSex, Hard: true,
				Because: "The published equations have separate forms, and there is no unisex one."},
			{Field: FieldWeightLog, Hard: true,
				Because: "The estimate is built on a real weight. Guessing one produces a number that reads exactly like a measured one."},
			{Field: FieldSchedule,
				Because: "How often you train carries most of the error in this estimate. Without it I assume three sessions a week, and I would rather you knew that than not."},
		},
	},
	{
		Tool:    "plan_session",
		Unlocks: "a session built for you rather than a generic one",
		Needs: []Requirement{
			{Field: FieldEquipment,
				Because: "Without it I can only suggest movements from what is already on your record."},
			{Field: FieldExperience,
				Because: "Without it the session is pitched at the middle, which is too much for some people and too little for others."},
			{Field: FieldGoal,
				Because: "Sets, reps and rest come from the goal. Without one I use the general prescription."},
			{Field: FieldInjuries,
				Because: "Without it I am assuming nothing hurts, and I would rather be told than assume."},
		},
	},
	{
		Tool:    "log_food",
		Unlocks: "a food log that can warn you",
		Needs: []Requirement{
			{Field: FieldAllergies,
				Because: "Without it nothing I suggest is checked against anything that would hurt you."},
			{Field: FieldDietPreference,
				Because: "Without it a suggestion may be something you do not eat."},
		},
	},
}

// Known returns the registry.
func Known() []Capability {
	out := make([]Capability, len(known))
	copy(out, known)
	return out
}
