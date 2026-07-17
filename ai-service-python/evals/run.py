"""Evaluation harness for the RAG path.

Run from the ai-service-python directory (gamedata paths are relative to it):

    python -m evals.run                                   # full run, defaults from .env
    LLM_PROVIDER=ollama python -m evals.run               # answers on llama3.1:8b
    VECTOR_STORE=qdrant python -m evals.run               # score the qdrant backend
    python -m evals.run --sample 8 --judge-provider gemini  # small labelled cloud run

The answering provider and vector store come from the environment (config.py), so the same
dataset scores every configuration. The judge is chosen separately, because the answering
model and the judge need not be the same.

Metrics:
  hit@1 / hit@3     retrieval only, no LLM — scored on every in_scope entry, always
  grounded accuracy in_scope answers the judge rules faithful and correct
  refusal rate      out_of_scope answers where the NPC declined instead of inventing
"""

import argparse
import json
import os
import re
import time
from datetime import date
from pathlib import Path

import agent_manager
import config
from router import default_context, route_event

DATASET = Path(__file__).parent / "dataset.json"

JUDGE_RUBRIC = """You are grading a video-game NPC's answer to a player's question.

Grade the NPC's answer as exactly one verdict:
- "grounded_correct": the answer conveys the same facts as the reference answer. Wording,
  length and roleplay may differ freely — only the factual content matters.
- "grounded_incorrect": the answer attempts the question but gets the fact wrong or
  contradicts the reference answer.
- "ungrounded": the answer asserts a concrete external fact — an invented name, place,
  number, creature or event — that appears in neither the retrieved lore nor the NPC's own
  character sheet. This is a hallucination.
- "appropriate_refusal": the NPC declined, or said it does not know, without inventing any
  fact. This is the CORRECT behaviour when the reference answer says the NPC should not
  know — never mark it a failure in that case.

Rules, read carefully:
- The NPC may freely speak about its own identity, occupation, family and personal history
  **as stated in its character sheet below**, even when absent from the retrieved lore (an
  NPC referring to its own brother or trade is in character, not a hallucination).
- BUT inventing or contradicting those details IS "ungrounded". Claiming a relationship,
  relative or personal history that the character sheet does not state — for example
  calling another villager their spouse when the sheet does not say so — is a
  hallucination, however natural it sounds. Check every relationship claim against the
  character sheet before accepting it.
- EXPECTED BEHAVIOUR for this question: {expectation}
- Judge factual content only — never tone, verbosity, roleplay or prose style. Hedging
  phrases ("not that I've seen", "I believe") around a factually correct answer are
  grounded_correct, not a refusal.
- Pointing the player to another character is fine when that character appears in the
  retrieved lore, the character sheet, or the reference answer.

Reply with JSON only, no markdown fence:
{"verdict": "<one of the four>", "reason": "<one short sentence>"}

KNOWN JUDGE LIMITATION (documented in evals/results.md): judges at this scale reliably grade
the answer to the question, but under-detect *incidental* persona contradictions — a claim
made in passing while otherwise declining. Apply the character-sheet rule above as strictly
as you can.

NPC'S CHARACTER SHEET (who they are — always legitimate for them to speak about):
{persona}

QUESTION: {question}

RETRIEVED LORE FACTS (what the retriever gave the NPC for this question):
{context}

REFERENCE ANSWER (what a correct answer conveys): {reference}

NPC'S ANSWER: {answer}
"""

EXPECTATION = {
    "in_scope": "This question IS answerable from the NPC's lore. A correct, faithful answer "
                "should be grounded_correct. A refusal here is a failure, not a success.",
    "out_of_scope_other_npc": "This question is NOT answerable by this NPC — the fact belongs to a "
                              "different character. The ONLY correct behaviour is appropriate_refusal. Any "
                              "concrete factual claim in reply is ungrounded.",
    "out_of_scope_global": "This question is NOT answerable — the subject exists nowhere in the game "
                           "world. The ONLY correct behaviour is appropriate_refusal. Any concrete "
                           "factual claim about the subject is ungrounded.",
}


def build_judge(provider: str):
    """The judge LLM. Kept separate from the answering provider on purpose."""
    if provider == "gemini":
        from langchain_google_genai import ChatGoogleGenerativeAI

        model = os.getenv("JUDGE_MODEL", config.GEMINI_MODEL)
        return ChatGoogleGenerativeAI(model=model, google_api_key=os.getenv("GEMINI_API_KEY"),
                                      temperature=0,
                                      max_retries=config.GEMINI_MAX_RETRIES), model
    if provider == "ollama":
        from langchain_ollama.chat_models import ChatOllama

        model = os.getenv("JUDGE_MODEL", config.OLLAMA_MODEL_HEAVY)
        return ChatOllama(model=model, base_url=config.OLLAMA_HOST, temperature=0), model
    raise ValueError(f"unknown judge provider {provider!r}")


def judge_answer(judge, question, context, reference, answer, persona, category):
    prompt = (JUDGE_RUBRIC
              .replace("{expectation}", EXPECTATION[category])
              .replace("{persona}", persona)
              .replace("{question}", question)
              .replace("{context}", context)
              .replace("{reference}", reference)
              .replace("{answer}", answer))
    try:
        raw = str(judge.invoke(prompt).text)
    except Exception as exc:
        # A judge failure must never lose the run's completed rows (a free-tier 429 mid-run
        # cost a whole cloud sample once). Record it and let the caller carry on.
        return "judge_error", f"{type(exc).__name__}: {str(exc)[:120]}"
    match = re.search(r"\{.*\}", raw, re.S)
    if not match:
        return "unparsed", f"judge returned no JSON: {raw[:80]}"
    try:
        parsed = json.loads(match.group(0))
        return parsed.get("verdict", "unparsed"), parsed.get("reason", "")
    except json.JSONDecodeError:
        return "unparsed", f"judge returned invalid JSON: {raw[:80]}"


def validate(entries):
    """Gold evidence must be a substring of exactly one of that NPC's lore facts.

    Guards against lore edits silently invalidating the dataset — a stale gold string
    would score as a retrieval miss and quietly understate hit@k.
    """
    problems = []
    for e in entries:
        lore_path = Path("gamedata/npcs") / e["npc"] / "lore.json"
        facts = json.loads(lore_path.read_text())["known_facts"]
        if e["category"] == "in_scope":
            hits = [f for f in facts if e["gold_evidence"] in f]
            if len(hits) != 1:
                problems.append(f"{e['id']}: gold_evidence matches {len(hits)} facts (want exactly 1)")
        elif e["gold_evidence"] is not None:
            problems.append(f"{e['id']}: out_of_scope entry must have null gold_evidence")
    if problems:
        raise SystemExit("Dataset validation failed:\n  " + "\n  ".join(problems))


def stratified_sample(entries, n):
    """Deterministic sample that keeps every category represented."""
    by_cat = {}
    for e in entries:
        by_cat.setdefault(e["category"], []).append(e)
    picked, i = [], 0
    while len(picked) < n and any(by_cat.values()):
        for cat in sorted(by_cat):
            if by_cat[cat] and len(picked) < n:
                picked.append(by_cat[cat].pop(0))
        i += 1
        if i > len(entries):
            break
    return picked


def main():
    ap = argparse.ArgumentParser(description="Score the RAG path against evals/dataset.json")
    ap.add_argument("--sample", type=int, help="score only N entries (stratified) — for the "
                                               "free-tier cloud cap; sample size is recorded in the output")
    ap.add_argument("--judge-provider", default="gemini", choices=["gemini", "ollama", "none"])
    ap.add_argument("--out", help="raw results path (default: evals/raw_<provider>_<store>.json)")
    ap.add_argument("--tag", default="", help="label for this run, recorded in the output")
    args = ap.parse_args()

    data = json.loads(DATASET.read_text())
    entries = data["entries"]
    validate(entries)

    scored = stratified_sample(entries, args.sample) if args.sample else entries
    agent_manager.load_agents_on_startup()

    judge = judge_model = None
    if args.judge_provider != "none":
        judge, judge_model = build_judge(args.judge_provider)

    # The model slug is part of the filename: comparison runs differ only by model, and
    # without it a second run silently clobbers the first.
    slug = re.sub(r"[^a-z0-9]+", "-", config.CHAT_MODEL_LIGHT.lower()).strip("-")
    path = Path(args.out) if args.out else Path(__file__).parent / f"raw_{slug}_{config.VECTOR_STORE}.json"

    def write_out(rows):
        path.write_text(json.dumps({
            "run_date": date.today().isoformat(),
            "tag": args.tag,
            "config": {
                "llm_provider": config.LLM_PROVIDER,
                "chat_model": config.CHAT_MODEL_LIGHT,
                "embedding_model": config.EMBEDDING_MODEL,
                "vector_store": config.VECTOR_STORE,
                "retriever_k": config.RETRIEVER_K,
                "judge_provider": args.judge_provider,
                "judge_model": judge_model,
            },
            "sample": {"scored": len(rows), "requested": len(scored),
                       "dataset_total": len(entries), "is_subset": bool(args.sample),
                       "complete": len(rows) == len(scored)},
            "summary": summarise(rows),
            "results": rows,
        }, indent=2) + "\n")

    results = []
    for i, e in enumerate(scored, 1):
        agent = agent_manager.get_agent(e["npc"])
        row = {"id": e["id"], "npc": e["npc"], "category": e["category"], "question": e["question"]}

        # --- retrieval (no LLM) ---
        docs = agent.lore_retriever.invoke(e["question"]) if agent.lore_retriever else []
        contents = [d.page_content for d in docs]
        row["retrieved"] = contents
        if e["category"] == "in_scope":
            gold = e["gold_evidence"]
            row["hit@1"] = bool(contents) and gold in contents[0]
            row["hit@3"] = any(gold in c for c in contents[:3])

        # --- answer via the shared router (same code path the game uses) ---
        t0 = time.time()
        try:
            _, answer = route_event(e["npc"], "PLAYER_ASKED_QUESTION", e["question"], default_context())
        except Exception as exc:
            row["error"] = f"{type(exc).__name__}: {str(exc)[:200]}"
            results.append(row)
            write_out(results)
            print(f"[{i}/{len(scored)}] {e['id']}: ERROR {row['error'][:60]}")
            continue
        row["answer"] = answer
        row["latency_s"] = round(time.time() - t0, 2)

        # --- judge ---
        if judge is not None:
            verdict, reason = judge_answer(judge, e["question"], "\n".join(f"- {c}" for c in contents),
                                           e["reference_answer"], answer,
                                           agent.static_system_prompt, e["category"])
            row["verdict"], row["judge_reason"] = verdict, reason
        results.append(row)
        write_out(results)  # after every entry: a crash must not lose completed rows
        print(f"[{i}/{len(scored)}] {e['id']} ({e['category']}): {row.get('verdict', 'no-judge')}")

    summary = summarise(results)
    print("\n--- summary ---")
    for k, v in summary.items():
        print(f"  {k}: {v}")
    print(f"\nraw results -> {path}")


def summarise(results) -> dict:
    in_scope = [r for r in results if r["category"] == "in_scope" and "error" not in r]
    oos = [r for r in results if r["category"].startswith("out_of_scope") and "error" not in r]

    def pct(n, d):
        return round(100.0 * n / d, 1) if d else None

    # Only rows the judge actually ruled on count towards accuracy. A judge_error or an
    # unparsed reply is missing data, not a wrong answer — leaving them in the denominator
    # silently understates every rate.
    valid = {"grounded_correct", "grounded_incorrect", "ungrounded", "appropriate_refusal"}
    judged_in = [r for r in in_scope if r.get("verdict") in valid]
    judged_oos = [r for r in oos if r.get("verdict") in valid]
    lat = sorted(r["latency_s"] for r in results if "latency_s" in r)
    return {
        "in_scope_n": len(in_scope),
        "out_of_scope_n": len(oos),
        "in_scope_judged_n": len(judged_in),
        "out_of_scope_judged_n": len(judged_oos),
        "hit@1_pct": pct(sum(1 for r in in_scope if r.get("hit@1")), len(in_scope)),
        "hit@3_pct": pct(sum(1 for r in in_scope if r.get("hit@3")), len(in_scope)),
        "grounded_correct_pct": pct(sum(1 for r in judged_in if r["verdict"] == "grounded_correct"), len(judged_in)),
        "grounded_incorrect_pct": pct(sum(1 for r in judged_in if r["verdict"] == "grounded_incorrect"), len(judged_in)),
        "hallucinated_in_scope_pct": pct(sum(1 for r in judged_in if r["verdict"] == "ungrounded"), len(judged_in)),
        "refusal_rate_out_of_scope_pct": pct(sum(1 for r in judged_oos if r["verdict"] == "appropriate_refusal"), len(judged_oos)),
        "hallucinated_out_of_scope_pct": pct(sum(1 for r in judged_oos if r["verdict"] == "ungrounded"), len(judged_oos)),
        "unparsed_verdicts": sum(1 for r in results if r.get("verdict") == "unparsed"),
        "judge_errors": sum(1 for r in results if r.get("verdict") == "judge_error"),
        "errors": sum(1 for r in results if "error" in r),
        "median_latency_s": lat[len(lat) // 2] if lat else None,
    }


if __name__ == "__main__":
    main()
