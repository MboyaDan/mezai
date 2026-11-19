// Sample content for the pilot demo (brief section 18). Everyday questions
// and answers written for MezAI's sample data (CLAUDE.md seed rules): each
// topic has a helpful answer and a weak one, so a careful contributor can
// always tell which is better.

type Topic = { task: string; good: string; weak: string };

const topics: Topic[] = [
  {
    task: "keep my phone battery healthy",
    good: "Avoid running it to zero often, keep it out of strong heat and use a good charger.",
    weak: "Charge it whenever you like, batteries do not wear out.",
  },
  {
    task: "save a little money each month",
    good: "Decide an amount on payday, move it to a separate account first and track small daily spending.",
    weak: "Just spend less, it is easy.",
  },
  {
    task: "sleep better at night",
    good: "Keep a regular bedtime, put the phone away an hour before and avoid heavy meals late.",
    weak: "Drink coffee in the evening so you are tired later.",
  },
  {
    task: "keep tomatoes fresh for longer",
    good: "Store ripe ones in a cool place out of direct sun and use the softest first.",
    weak: "Wash them all and leave them wet in a closed bag.",
  },
  {
    task: "prepare for a job interview",
    good: "Read about the organisation, practise answers about your experience and arrive early.",
    weak: "Do not prepare, interviews are mostly luck.",
  },
  {
    task: "write a polite email to a teacher",
    good: "Greet them by name, say who you are, ask your question clearly and thank them.",
    weak: "Write in capital letters so they notice it.",
  },
  {
    task: "learn new words in a language",
    good: "Learn a few words each day, use them in sentences and review them the next day.",
    weak: "Read a whole dictionary once.",
  },
  {
    task: "keep a kitchen clean",
    good: "Wipe surfaces after cooking, wash dishes the same day and empty the bin often.",
    weak: "Clean it once a month, that is enough.",
  },
  {
    task: "stay safe when crossing a busy road",
    good: "Use a crossing where there is one, look both ways and wait until vehicles stop.",
    weak: "Run across quickly while looking at your phone.",
  },
  {
    task: "plan a small birthday party",
    good: "Set a budget, invite guests a week ahead and plan simple food and music.",
    weak: "Invite everyone on the day and hope it works.",
  },
  {
    task: "protect my mobile money PIN",
    good: "Never share it, cover the keypad when typing and change it if someone may have seen it.",
    weak: "Write it on the back of your phone so you do not forget.",
  },
  {
    task: "keep houseplants alive",
    good: "Water when the soil is dry, give them light and do not leave water standing in the pot.",
    weak: "Water them three times a day.",
  },
  {
    task: "start running for exercise",
    good: "Begin with short walks and slow runs, rest between days and build up each week.",
    weak: "Run as far as you can on the first day.",
  },
  {
    task: "make a simple shopping list",
    good: "Check what you have, write what you need by type and stick to it in the shop.",
    weak: "Buy whatever looks nice in the shop.",
  },
  {
    task: "reduce plastic use at home",
    good: "Carry a reusable bag, buy loose fruit and reuse containers.",
    weak: "Plastic is fine, use as much as you want.",
  },
  {
    task: "help a child with homework",
    good: "Sit with them, ask what they understand and let them try before you help.",
    weak: "Do the homework for them so it is finished.",
  },
  {
    task: "take care of a new pair of shoes",
    good: "Clean them gently, let them dry in the air and store them in a dry place.",
    weak: "Dry them on a hot stove.",
  },
  {
    task: "remember to drink enough water",
    good: "Keep a bottle nearby, drink a glass with each meal and refill it during the day.",
    weak: "Only drink when you feel very thirsty.",
  },
  {
    task: "keep my passwords safe",
    good: "Use a different long password for each account and turn on two-step sign-in.",
    weak: "Use the same short password everywhere.",
  },
  {
    task: "organise my study time",
    good: "Make a weekly timetable, study in short blocks and take small breaks.",
    weak: "Study everything the night before the exam.",
  },
  {
    task: "cook rice that is not sticky",
    good: "Rinse the rice, use the right amount of water and let it rest covered after cooking.",
    weak: "Stir it all the time while it boils.",
  },
  {
    task: "travel by bus with a lot of luggage",
    good: "Pack in one or two bags, label them and arrive early to load them.",
    weak: "Carry many small bags and arrive at the last minute.",
  },
  {
    task: "talk to a neighbour about noise",
    good: "Choose a calm moment, explain the problem politely and suggest a simple solution.",
    weak: "Shout at their door late at night.",
  },
  {
    task: "keep a small shop's records",
    good: "Write every sale and purchase each day and check the totals at the end of the week.",
    weak: "Keep the numbers in your head.",
  },
  {
    task: "prepare for a long power cut",
    good: "Charge phones and a lamp early, keep a torch ready and store food that needs no fridge.",
    weak: "Open the fridge often to check the food.",
  },
  {
    task: "make tea for guests",
    good: "Boil fresh water, ask how they like it and serve it with a small snack.",
    weak: "Serve yesterday's cold tea.",
  },
  {
    task: "share a car ride safely",
    good: "Check the driver and car details, share your trip with a friend and sit in the back.",
    weak: "Get into any car that stops for you.",
  },
  {
    task: "write a short thank-you note",
    good: "Name what you are thankful for, keep it brief and sign it warmly.",
    weak: "Send a blank card.",
  },
  {
    task: "care for a cold at home",
    good: "Rest, drink warm fluids and see a health worker if it gets worse or lasts long.",
    weak: "Ignore it and do heavy work all day.",
  },
  {
    task: "keep a bicycle in good order",
    good: "Pump the tyres, oil the chain and check the brakes before long rides.",
    weak: "Never check the brakes.",
  },
];

const asks = (t: string) => [`How can I ${t}?`, `Any simple advice to ${t}?`, `What is an easy way to ${t}?`];

/** Which response is the helpful one, by item. */
export type Pair = {
  external_id: string;
  prompt: string;
  response_a: string;
  response_b: string;
  better: "a" | "b";
};

function pair(id: string, prompt: string, t: Topic, aIsGood: boolean): Pair {
  return {
    external_id: id,
    prompt,
    response_a: aIsGood ? t.good : t.weak,
    response_b: aIsGood ? t.weak : t.good,
    better: aIsGood ? "a" : "b",
  };
}

/** 50 items: every topic asked two ways (60), the first 50 kept. */
export function demoItems(run: string): Pair[] {
  const out: Pair[] = [];
  topics.forEach((t, i) =>
    asks(t.task)
      .slice(0, 2)
      .forEach((p, j) => out.push(pair(`${run}-i${i}-${j}`, p, t, (i + j) % 2 === 0))),
  );
  return out.slice(0, 50);
}

/** 10 gold items: ten topics asked a third way, with the expected preference. */
export function demoGold(run: string): Pair[] {
  return topics.slice(0, 10).map((t, i) => pair(`${run}-g${i}`, asks(t.task)[2], t, i % 2 === 1));
}

const content = (p: Pair) => ({
  external_id: p.external_id,
  prompt: p.prompt,
  response_a: p.response_a,
  response_b: p.response_b,
});

export const jsonl = (rows: Pair[]) => rows.map((p) => JSON.stringify(content(p))).join("\n") + "\n";

export const goldJsonl = (rows: Pair[]) =>
  rows.map((p) => JSON.stringify({ ...content(p), expected: { preference: p.better } })).join("\n") + "\n";
