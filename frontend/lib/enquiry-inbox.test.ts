import { describe, expect, it } from "vitest";
import { projectTypes, timelines, volumes } from "@/content/contact";
import {
  buttonMoves,
  change,
  enquiryStatuses,
  moveLabel,
  projectTypeLabel,
  statusText,
  timelineLabel,
  volumeLabel,
} from "./enquiry-inbox";

const me = "11111111-1111-7111-8111-111111111111";
const other = "22222222-2222-7222-8222-222222222222";
const org = "33333333-3333-7333-8333-333333333333";
const at = (
  status: (typeof enquiryStatuses)[number],
  assigned_to: string | null = null,
  org_id: string | null = null,
) => ({
  status,
  assigned_to,
  org_id,
});

describe("change", () => {
  it("assigns a new enquiry when someone is picked", () => {
    expect(change(at("new"), { assigned_to: other }, me)).toEqual({
      expected_status: "new",
      status: "assigned",
      assigned_to: other,
      org_id: null,
    });
  });
  it("unassigns back to new", () => {
    expect(change(at("assigned", other), { status: "new" }, me)).toMatchObject({
      status: "new",
      assigned_to: null,
    });
    expect(change(at("assigned", other), { assigned_to: null }, me)).toMatchObject({
      status: "new",
      assigned_to: null,
    });
  });
  it("keeps the assignee when the conversation starts and takes it on when nobody has it", () => {
    expect(change(at("assigned", other), { status: "in_conversation" }, me).assigned_to).toBe(other);
    expect(change(at("lost"), { status: "in_conversation" }, me).assigned_to).toBe(me);
  });
  it("links an organisation only when won", () => {
    expect(change(at("in_conversation", me), { status: "won", org_id: org }, me)).toMatchObject({
      status: "won",
      org_id: org,
    });
    expect(change(at("in_conversation", me, null), { status: "lost", org_id: org }, me).org_id).toBeNull();
    expect(change(at("won", me, org), { org_id: null }, me)).toMatchObject({ status: "won", org_id: null });
  });
  it("sends the status it saw", () => {
    expect(change(at("spam"), { status: "new" }, me)).toMatchObject({
      expected_status: "spam",
      status: "new",
    });
  });
});

describe("labels", () => {
  it("names every status and move", () => {
    for (const s of enquiryStatuses) expect(statusText[s].label).toBeTruthy();
    expect(moveLabel("spam", "new")).toBe("Not spam");
    expect(moveLabel("assigned", "new")).toBe("Unassign");
    expect(moveLabel("lost", "in_conversation")).toBe("Reopen");
    expect(buttonMoves(["assigned", "lost", "spam"])).toEqual(["lost", "spam"]);
  });
  it("reads the contact form's choices", () => {
    expect(projectTypeLabel(projectTypes[0].value)).toBe(projectTypes[0].label);
    expect(volumeLabel(volumes[0].value)).toBe(volumes[0].label);
    expect(timelineLabel(timelines[0].value)).toBe(timelines[0].label);
    expect(volumeLabel("")).toBe("Not given");
  });
});
