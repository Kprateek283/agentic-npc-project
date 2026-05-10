# Interview Day Checklist - January 14, 2026, 8:30 PM IST

## Pre-Interview Setup (1 hour before)

### Technical Setup
- [ ] Test internet connection (run speed test)
- [ ] Test Google Meet link: meet.google.com/wnm-vtsm-sqg
- [ ] Test camera and microphone
- [ ] Close unnecessary applications (Slack, email, Discord)
- [ ] Charge laptop (plug in charger)
- [ ] Have phone nearby (silent mode) as backup for Meet
- [ ] Open VS Code with project loaded
- [ ] Have architecture diagram ready (draw on paper/whiteboard if available)

### Environment Setup
- [ ] Quiet room with good lighting
- [ ] Glass of water nearby
- [ ] Notebook and pen for taking notes/drawing
- [ ] Turn off all notifications (phone, laptop, smart watch)
- [ ] Inform family/roommates not to disturb

### Documents to Have Open
- [ ] INTERVIEW_CHEATSHEET.md (quick reference)
- [ ] ARCHITECTURE_DIAGRAM.md (visual aid)
- [ ] README.md (project overview)
- [ ] This checklist

### Mental Preparation
- [ ] Review 30-second elevator pitch (practice 3 times)
- [ ] Review key design decisions
- [ ] Review security & scalability talking points
- [ ] Take deep breaths, stay calm
- [ ] Remember: They want you to succeed!

---

## Opening (First 5 Minutes)

### Greeting
"Hi Jonathan and Anikate! Great to meet you both. I'm excited to discuss my projects and learn more about SkippyEd."

### Elevator Pitch (When Asked)
"I built an intelligent NPC framework for games using a microservices architecture. The system enables NPCs to have persistent memory, dynamic emotions, and context-aware conversations powered by AI. It's built with a Go backend for orchestration and performance, a Python service for AI/LLM processing via gRPC, PostgreSQL for persistent state, Redis for caching, and uses LangChain with RAG to generate unique dialogue grounded in game lore."

---

## Project Discussion (20 minutes)

### Key Talking Points (Use STAR Method)

#### Why I Built This
**Situation**: "I'm passionate about game AI. Most NPCs in games have scripted dialogue that feels repetitive."
**Task**: "I wanted to build NPCs that feel alive - remembering past interactions and responding naturally."
**Action**: "I designed a system with persistent memory, emotion tracking, and LLM-powered dialogue."
**Result**: "NPCs can handle unscripted conversations, remember player actions, and their mood changes dynamically."

#### Architecture Decision: Microservices
**Why Two Services?**
"I chose polyglot microservices to leverage each language's strengths:
- Go excels at: concurrency (handling thousands of WebSocket connections), performance, and database operations
- Python excels at: ML/AI libraries (LangChain ecosystem), rapid prototyping
- Alternative was Python FastAPI for everything, but Go's performance for I/O-heavy operations won out."

**Why gRPC Over REST?**
- 7-10x faster (binary Protocol Buffers vs JSON)
- Type-safe contract (auto-generated code)
- Streaming support (future enhancement)
- Trade-off: More complex than REST, but worth it for internal services

#### Technical Challenge: Agent Cold-Start Latency
**Problem**: "Initially, loading an agent on first request took 2-3 seconds (loading files, building vector store)."
**Solution**: "Pre-load all agents at startup into a singleton registry. Now first request is just as fast as subsequent ones (~300ms)."
**Learning**: "Optimize for hot paths - move expensive operations to initialization."

#### Scalability Design
**Current State**: "System handles ~1000 concurrent players with 300-500ms response time."

**To Scale to 10K Players**:
- Horizontal scaling: 5 Go backend replicas behind load balancer
- 3 Python AI service replicas with gRPC load balancing
- Redis cluster for distributed caching
- PostgreSQL read replicas

**To Scale to 100K Players**:
- Auto-scaling with Kubernetes HPA
- Message queue (Kafka) for async AI processing
- Database sharding by player_id
- Distributed vector database (Qdrant/Pinecone)

#### Security Considerations
**Current**:
- Environment variables for secrets
- Ent ORM prevents SQL injection
- Input validation with DTO schemas
- Emotion value clamping (prevents overflow)

**Would Add**:
- JWT-based authentication (currently simple username check)
- Rate limiting (prevent DoS attacks)
- Prompt injection protection (sanitize player input)
- TLS encryption for all services

#### Best Practices
- **Clean Architecture**: Clear layer separation (API → Domain → Infrastructure)
- **Dependency Injection**: All dependencies passed explicitly (testable)
- **Error Handling**: Always wrap errors with context
- **Type Safety**: Protocol Buffers, Ent ORM, type hints
- **Graceful Shutdown**: `defer` cleanup in main()

---

## Code Review Exercise (10 minutes)

### Approach
1. **Read carefully**: Take 30 seconds to understand the code
2. **Ask questions**: "What's the expected behavior here?" or "What scale are we targeting?"
3. **Think aloud**: "Let me walk through what this code does..."
4. **Identify issues**: "I see a potential race condition/N+1 query/memory leak..."
5. **Propose solutions**: Give 2-3 options with trade-offs
6. **Be practical**: Choose the simplest solution that works

### Common Bug Patterns to Look For
- Race conditions (concurrent access to shared state)
- N+1 queries (looping database calls)
- Error handling (ignoring errors, not wrapping with context)
- Memory leaks (not cleaning up resources)
- Security vulnerabilities (SQL injection, prompt injection)

### If Stuck
- "Let me think through the data flow..."
- "What happens when X occurs?"
- "I'd verify this with a unit test that..."
- "I'm not sure, but I'd research approaches like X and Y"

---

## System Design Exercise (10 minutes)

### Framework: RADIO

**R - Requirements**
- "Let me clarify the requirements..."
- Functional: Core features
- Non-functional: Scale, latency, uptime

**A - Architecture**
- Draw high-level diagram
- Identify key components
- Choose technologies

**D - Data Model**
- Sketch database schema
- Identify relationships
- Consider indexes

**I - Implementation**
- Code snippets for critical flows
- API contracts
- Deployment strategy

**O - Optimizations**
- Caching strategy
- Scaling plan
- Monitoring/observability

### Trade-offs to Discuss
- SQL vs NoSQL
- Synchronous vs Asynchronous
- Strong consistency vs Eventual consistency
- Vertical vs Horizontal scaling
- Cost vs Performance

### Example: "Design a leaderboard system"

**Quick Answer**:
"I'd use Redis sorted sets for real-time rankings, PostgreSQL for persistence, and cache top 100 players. For global leaderboard with millions of players, I'd pre-compute rankings hourly and use pagination. Trade-off: Real-time accuracy vs performance."

---

## Questions to Ask Them (5 minutes)

### About the Role
1. "What does a typical day look like for your engineering team?"
2. "What are the biggest technical challenges SkippyEd is facing right now?"
3. "How much autonomy do interns have on architectural decisions?"
4. "What's your approach to code review and quality?"

### About the Team
1. "How large is the engineering team, and how is it structured?"
2. "What's the balance between frontend and backend work in this role?"
3. "How do you support learning and professional development?"
4. "What technologies is the team most excited about?"

### About the Company
1. "Where do you see SkippyEd's product in 1-2 years?"
2. "How do you balance moving fast vs building robust systems?"
3. "What's the deployment process - CI/CD, testing, etc.?"
4. "What makes SkippyEd a great place to work?"

### Red Flags (Ask Diplomatically)
- "How do you manage technical debt?"
- "What's the on-call rotation like?"
- "How do you handle work-life balance during crunch times?"

---

## Closing (5 minutes)

### Thank You
"Thank you both for your time. I really enjoyed discussing my projects and learning about SkippyEd. I'm excited about the opportunity to contribute to your team."

### Follow-up
- Ask about next steps: "What's the timeline for the next steps in the process?"
- Reiterate interest: "I'm very interested in this role and would love to join the team."
- Contact info: "If you need any additional information, feel free to reach out."

---

## Post-Interview (Within 24 hours)

### Send Thank You Email
```
Subject: Thank you - Technical Interview Follow-up

Hi Jonathan and Anikate,

Thank you for taking the time to interview me yesterday for the full-time internship role at SkippyEd. I enjoyed discussing my projects, particularly the Agentic NPC Framework, and learning about the exciting challenges you're tackling.

Our conversation about [specific topic discussed] reinforced my interest in joining your team. I'm particularly excited about [something specific they mentioned about the role/company].

If there's any additional information I can provide, please don't hesitate to reach out. I look forward to hearing about the next steps.

Best regards,
Prateek
```

### Reflect & Document
- [ ] Write down questions you struggled with
- [ ] Note topics to study further
- [ ] Document what went well
- [ ] Prepare for potential follow-up round

---

## Emergency Scenarios

### Technical Issues During Interview

**Internet Drops**:
1. Quickly rejoin via phone (have Meet app ready)
2. Apologize briefly, continue

**Computer Crashes**:
1. Have phone with Meet app as backup
2. Rejoin immediately
3. "Sorry about that, technical issue. Where were we?"

**Can't Hear/See Them**:
1. Check your audio settings
2. Use chat: "I'm having audio issues, can we try reconnecting?"
3. Don't waste time troubleshooting - move to phone if needed

### Mental Blocks During Questions

**Drawing a Blank**:
1. Take a breath
2. "Give me a moment to think this through..."
3. Think aloud: "Let me break this down..."
4. Start with what you DO know

**Don't Know the Answer**:
✅ "I haven't encountered this before, but here's how I'd approach it..."
✅ "I'm not familiar with [technology], but it sounds similar to [something you know]..."
❌ Don't make up answers or pretend

**Nervous/Anxious**:
1. Remember: You built something impressive
2. They're evaluating fit, not perfection
3. It's okay to be human (pause, say "good question")
4. Focus on your breath (slow, deep breaths)

---

## Confidence Boosters (Read Before Interview)

### What You've Accomplished
✅ Built a production-grade system with microservices architecture
✅ Used industry-standard tools: Go, Python, gRPC, PostgreSQL, Redis, Docker
✅ Solved real scaling challenges (caching, agent pre-loading, gRPC optimization)
✅ Demonstrated polyglot skills (Go + Python + SQL + Protocol Buffers)
✅ Thought about non-functionals (security, monitoring, deployment)

### Why You're a Strong Candidate
✅ You've built complex systems end-to-end
✅ You understand trade-offs and design decisions
✅ You can articulate technical concepts clearly
✅ You're passionate about learning and problem-solving
✅ You've demonstrated initiative (built this without being asked)

### Mindset
- This is a **conversation**, not an interrogation
- They **want you to succeed** (hiring is hard!)
- It's okay to **think aloud** and ask questions
- **Authenticity** matters more than perfection
- You're also **evaluating them** (is this a good fit for you?)

---

## Quick Stats to Remember

### Your Project Numbers
- **Services**: 4 (Go, Python, PostgreSQL, Redis)
- **Lines of Code**: ~5,000 (Go), ~2,000 (Python)
- **Response Time**: 300ms (RAG), 1-2s (LangGraph)
- **Concurrent Connections**: Tested with 100+
- **Database Tables**: 8 entities
- **gRPC Methods**: 1 primary (Think), extensible
- **NPCs**: 8 fully configured characters

### Performance
- **WebSocket Latency**: 10-50ms
- **gRPC Overhead**: <5ms
- **Cache Hit Rate**: ~70%
- **DB Query Time**: ~20ms (with index)

---

## Final Reminders

### Do's ✅
- Be enthusiastic and curious
- Explain your thinking process
- Ask clarifying questions
- Admit when you don't know something
- Show passion for learning
- Smile and make eye contact
- Take notes during the conversation

### Don'ts ❌
- Don't ramble or go off-topic
- Don't criticize previous employers/projects
- Don't pretend to know things you don't
- Don't interrupt the interviewers
- Don't be overly casual or informal
- Don't check phone/notifications
- Don't speak negatively about any technology

---

## Breathing Exercise (If Nervous)

1. **5 seconds** - Breathe in slowly through nose
2. **5 seconds** - Hold your breath
3. **5 seconds** - Breathe out slowly through mouth
4. **Repeat 3 times**

This activates your parasympathetic nervous system and calms anxiety.

---

## Power Poses (2 minutes before interview)

Stand up, stretch, do a "power pose":
- Hands on hips, chest out (Wonder Woman pose)
- Arms raised in V (victory pose)
- Big stretch with arms overhead

Research shows this reduces cortisol (stress hormone) and increases confidence.

---

## Last Words

You've prepared thoroughly. You've built something impressive. You know your stuff.

**Trust yourself. Be authentic. Enjoy the conversation.**

**You've got this! 🚀**

---

*Interview Details:*
- **Date**: Tuesday, January 14, 2026
- **Time**: 8:30 PM - 9:15 PM IST
- **Link**: meet.google.com/wnm-vtsm-sqg
- **Interviewers**: Jonathan & Anikate (Technical Leads)
- **Duration**: 45 minutes
- **Format**: Projects discussion + Code review + System design

