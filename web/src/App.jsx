import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import './App.css'

const DEFAULT_LOC = { lat: 39.9042, lng: 116.4074 }

function useSpeechRecognition({ onResult }) {
  const recRef = useRef(null)
  const [supported, setSupported] = useState(false)
  const [listening, setListening] = useState(false)

  useEffect(() => {
    const SR = window.SpeechRecognition || window.webkitSpeechRecognition
    if (!SR) return
    setSupported(true)
    const rec = new SR()
    rec.lang = 'zh-CN'
    rec.interimResults = false
    rec.continuous = false
    rec.onresult = (e) => {
      const text = e.results?.[0]?.[0]?.transcript || ''
      if (text) onResult(text)
    }
    rec.onend = () => setListening(false)
    rec.onerror = () => setListening(false)
    recRef.current = rec
  }, [onResult])

  const toggle = () => {
    const rec = recRef.current
    if (!rec) return
    if (listening) {
      rec.stop()
      setListening(false)
      return
    }
    setListening(true)
    rec.start()
  }

  return { supported, listening, toggle }
}

async function discoverAgents({ lat, lng, q, category, tags, radiusKm }) {
  const params = new URLSearchParams({
    lat: String(lat),
    lng: String(lng),
    radius_km: String(radiusKm || 50),
  })
  if (q) params.set('q', q)
  if (category) params.set('category', category)
  if (tags) params.set('tags', tags)
  const res = await fetch(`/api/v1/agents/discover?${params}`)
  if (!res.ok) throw new Error(await res.text())
  const data = await res.json()
  return data.agents || []
}

async function reversePlace(lat, lng) {
  const params = new URLSearchParams({ lat: String(lat), lng: String(lng) })
  const res = await fetch(`/api/v1/geo/reverse?${params}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export default function App() {
  const [loc, setLoc] = useState(DEFAULT_LOC)
  const [placeName, setPlaceName] = useState('北京')
  const [locHint, setLocHint] = useState('默认位置')
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('')
  const [tag, setTag] = useState('')
  const [agents, setAgents] = useState([])
  const [status, setStatus] = useState('')
  const [selected, setSelected] = useState(null)
  const [hostMessages, setHostMessages] = useState([])
  const [agentMessages, setAgentMessages] = useState([])
  const [hostInput, setHostInput] = useState('')
  const [agentInput, setAgentInput] = useState('')
  const [busy, setBusy] = useState(false)
  const [showScrollUp, setShowScrollUp] = useState(false)

  useEffect(() => {
    const onScroll = () => {
      setShowScrollUp(window.scrollY > window.innerHeight * 0.5)
    }
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  const onVoice = useCallback((text) => {
    setQuery(text)
    setStatus(`语音识别：${text}`)
  }, [])
  const speech = useSpeechRecognition({ onResult: onVoice })

  const resolvePlace = useCallback(async (lat, lng, hint) => {
    try {
      const place = await reversePlace(lat, lng)
      setPlaceName(place.name || place.city || '未知')
      setLocHint(hint || `${lat.toFixed(3)}, ${lng.toFixed(3)}`)
    } catch {
      setPlaceName(`${lat.toFixed(2)}, ${lng.toFixed(2)}`)
      setLocHint(hint || '坐标')
    }
  }, [])

  const useBeijingDemo = useCallback(() => {
    setLoc(DEFAULT_LOC)
    resolvePlace(DEFAULT_LOC.lat, DEFAULT_LOC.lng, '演示位置（示例 Agent 在北京）')
  }, [resolvePlace])

  const runDiscover = useCallback(async (overrides = {}) => {
    setStatus('正在发现附近 Agent…')
    const q = overrides.q ?? query
    const cat = overrides.category ?? category
    const tags = overrides.tags ?? tag
    const center = overrides.lat != null
      ? { lat: overrides.lat, lng: overrides.lng }
      : loc
    try {
      let list = await discoverAgents({
        lat: center.lat,
        lng: center.lng,
        q,
        category: cat,
        tags,
        radiusKm: overrides.radiusKm ?? 50,
      })
      // Demo agents are anchored in Beijing; if the browser is elsewhere, fall back.
      const farFromBeijing =
        Math.abs(center.lat - DEFAULT_LOC.lat) > 0.5 ||
        Math.abs(center.lng - DEFAULT_LOC.lng) > 0.5
      if (list.length === 0 && farFromBeijing && !overrides.skipFallback) {
        list = await discoverAgents({
          lat: DEFAULT_LOC.lat,
          lng: DEFAULT_LOC.lng,
          q,
          category: cat,
          tags,
          radiusKm: 50,
        })
        if (list.length > 0) {
          setLoc(DEFAULT_LOC)
          resolvePlace(DEFAULT_LOC.lat, DEFAULT_LOC.lng, '演示位置（示例 Agent 在北京）')
          setAgents(list)
          setStatus(`当前位置附近无 Agent；已切换到北京演示区，找到 ${list.length} 个`)
          return
        }
      }
      setAgents(list)
      setStatus(list.length ? `找到 ${list.length} 个 Agent` : '半径内没有匹配的 Agent（可点「北京演示」）')
    } catch (e) {
      setStatus(`发现失败：${e.message}`)
    }
  }, [loc, query, category, tag, resolvePlace])

  useEffect(() => {
    resolvePlace(DEFAULT_LOC.lat, DEFAULT_LOC.lng, '默认位置')
    if (!navigator.geolocation) {
      runDiscover()
      return
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        const next = { lat: pos.coords.latitude, lng: pos.coords.longitude }
        setLoc(next)
        resolvePlace(next.lat, next.lng, '当前位置')
      },
      () => {
        setLocHint('定位失败，使用默认位置')
        resolvePlace(DEFAULT_LOC.lat, DEFAULT_LOC.lng, '默认位置')
      },
      { enableHighAccuracy: false, timeout: 8000 },
    )
  }, [])

  useEffect(() => {
    runDiscover()
  }, [loc.lat, loc.lng])

  useEffect(() => {
    if (!query) return
    const t = setTimeout(() => runDiscover({ q: query }), 200)
    return () => clearTimeout(t)
  }, [query])

  const categories = useMemo(() => {
    const set = new Set(agents.map((a) => a.category).filter(Boolean))
    return Array.from(set)
  }, [agents])

  async function sendHost() {
    const message = hostInput.trim()
    if (!message || busy) return
    setBusy(true)
    setHostMessages((m) => [...m, { role: 'user', text: message }])
    setHostInput('')
    try {
      const res = await fetch('/api/v1/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message, lat: loc.lat, lng: loc.lng }),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || res.statusText)
      }
      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let assistant = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const chunks = buffer.split('\n\n')
        buffer = chunks.pop() || ''
        for (const chunk of chunks) {
          const line = chunk.trim()
          if (!line.startsWith('data:')) continue
          const payload = JSON.parse(line.slice(5).trim())
          if (payload.event === 'delta' && typeof payload.data === 'string') {
            assistant += payload.data
            setHostMessages((m) => {
              const copy = [...m]
              const last = copy[copy.length - 1]
              if (last?.role === 'assistant' && last.streaming) {
                copy[copy.length - 1] = { role: 'assistant', text: assistant, streaming: true }
              } else {
                copy.push({ role: 'assistant', text: assistant, streaming: true })
              }
              return copy
            })
          } else if (payload.event === 'message' && typeof payload.data === 'string') {
            assistant = payload.data
            setHostMessages((m) => {
              const copy = [...m]
              const last = copy[copy.length - 1]
              if (last?.role === 'assistant') {
                copy[copy.length - 1] = { role: 'assistant', text: assistant }
              } else {
                copy.push({ role: 'assistant', text: assistant })
              }
              return copy
            })
          } else if (payload.event === 'tool') {
            setHostMessages((m) => [
              ...m,
              { role: 'system', text: `调用工具 ${payload.data?.name || ''}` },
            ])
          } else if (payload.event === 'error') {
            setHostMessages((m) => [...m, { role: 'system', text: String(payload.data) }])
          }
        }
      }
    } catch (e) {
      setHostMessages((m) => [...m, { role: 'system', text: e.message }])
    } finally {
      setBusy(false)
    }
  }

  async function sendAgent() {
    if (!selected) return
    const message = agentInput.trim()
    if (!message || busy) return
    setBusy(true)
    setAgentMessages((m) => [...m, { role: 'user', text: message }])
    setAgentInput('')
    try {
      const res = await fetch(`/api/v1/a2a/${selected.id}/message:send`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || res.statusText)
      setAgentMessages((m) => [...m, { role: 'assistant', text: data.reply }])
    } catch (e) {
      setAgentMessages((m) => [...m, { role: 'system', text: e.message }])
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand-mark">
          <span className="brand-dot" aria-hidden />
          Agent Market
        </div>
        <div className="loc-pill" title={locHint}>
          <span className="loc-place">{placeName}</span>
          <span className="loc-hint">{locHint}</span>
        </div>
      </header>

      <section className="hero">
        <h1 className="hero-brand">
          Agent <span>Market</span>
        </h1>
        <p className="hero-lead">
          用语音搜索附近的 Agent。按标签、分类与距离发现，再通过 A2A 与它们对话。
        </p>
        <div className="search-row">
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="试试：附近的天气 / 美食 / 翻译 / 电影"
            onKeyDown={(e) => e.key === 'Enter' && runDiscover()}
          />
          <button className="btn btn-primary" type="button" onClick={() => runDiscover()}>
            搜索
          </button>
          {speech.supported && (
            <button
              className={`btn btn-voice ${speech.listening ? 'listening' : ''}`}
              type="button"
              onClick={speech.toggle}
            >
              {speech.listening ? '聆听中…' : '语音'}
            </button>
          )}
        </div>
        <div className="filters">
          <select value={category} onChange={(e) => setCategory(e.target.value)}>
            <option value="">全部分类</option>
            <option value="utility">utility</option>
            <option value="lifestyle">lifestyle</option>
            <option value="entertainment">entertainment</option>
            {categories.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
          <select value={tag} onChange={(e) => setTag(e.target.value)}>
            <option value="">全部标签</option>
            <option value="weather">weather</option>
            <option value="food">food</option>
            <option value="travel">travel</option>
            <option value="translate">translate</option>
            <option value="movie">movie</option>
            <option value="电影">电影</option>
          </select>
          <button className="btn btn-ghost" type="button" onClick={() => runDiscover()}>
            应用筛选
          </button>
          <button className="btn btn-ghost" type="button" onClick={useBeijingDemo}>
            北京演示
          </button>
        </div>
        <div className="status-line">{status}</div>
        <button
          className="hero-scroll"
          type="button"
          aria-label="向下翻一页"
          onClick={() =>
            window.scrollBy({ top: window.innerHeight, behavior: 'smooth' })
          }
        >
          <svg viewBox="0 0 24 24" width="28" height="28" aria-hidden>
            <path
              d="M6 9l6 6 6-6"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>
      </section>

      <section className="section" id="discover">
        <h2>附近的 Agent</h2>
        <p className="section-lead">按距离排序，支持标签与分类过滤。</p>
        <div className="agent-list">
          {agents.length === 0 && <div className="empty">暂无结果。请先启动示例 Agents，或放宽筛选条件。</div>}
          {agents.map((a, i) => (
            <article
              key={a.id}
              className="agent-item"
              style={{ animationDelay: `${i * 40}ms` }}
            >
              <div>
                <h3>{a.name}</h3>
                <p>{a.description}</p>
                <div className="meta">
                  {a.category && <span className="tag cat">{a.category}</span>}
                  {(a.tags || []).map((t) => (
                    <span className="tag" key={t}>
                      {t}
                    </span>
                  ))}
                </div>
              </div>
              <div style={{ textAlign: 'right', display: 'grid', gap: '0.5rem' }}>
                <div className="dist">{a.distance_km?.toFixed?.(2) ?? a.distance_km} km</div>
                <button
                  className="btn btn-ghost"
                  type="button"
                  onClick={() => {
                    setSelected(a)
                    setAgentMessages([{ role: 'system', text: `已连接 ${a.name}` }])
                  }}
                >
                  对话
                </button>
              </div>
            </article>
          ))}
        </div>
      </section>

      <section className="section" id="chat">
        <h2>对话</h2>
        <p className="section-lead">左侧与 Market Host 聊天（可自动发现并 A2A 调用）；右侧直连某个 Agent。</p>
        <div className="chat-grid">
          <div className="panel">
            <h3>Market Host</h3>
            <div className="messages">
              {hostMessages.length === 0 && (
                <div className="empty">例如：帮我找附近的天气 Agent，并问问今天天气</div>
              )}
              {hostMessages.map((m, idx) => (
                <div key={idx} className={`bubble ${m.role}`}>
                  {m.text}
                </div>
              ))}
            </div>
            <div className="composer">
              <input
                value={hostInput}
                onChange={(e) => setHostInput(e.target.value)}
                placeholder="对 Host 说…"
                onKeyDown={(e) => e.key === 'Enter' && sendHost()}
              />
              <button className="btn btn-primary" type="button" disabled={busy} onClick={sendHost}>
                发送
              </button>
            </div>
          </div>

          <div className="panel">
            <h3>{selected ? selected.name : '直连 Agent'}</h3>
            <div className="messages">
              {!selected && <div className="empty">从上方列表选择一个 Agent</div>}
              {agentMessages.map((m, idx) => (
                <div key={idx} className={`bubble ${m.role}`}>
                  {m.text}
                </div>
              ))}
            </div>
            <div className="composer">
              <input
                value={agentInput}
                onChange={(e) => setAgentInput(e.target.value)}
                placeholder={selected ? '经 A2A 发送…' : '先选择 Agent'}
                disabled={!selected}
                onKeyDown={(e) => e.key === 'Enter' && sendAgent()}
              />
              <button
                className="btn btn-primary"
                type="button"
                disabled={!selected || busy}
                onClick={sendAgent}
              >
                发送
              </button>
            </div>
          </div>
        </div>
      </section>

      {showScrollUp && (
        <button
          className="page-scroll-up"
          type="button"
          aria-label="向上翻一页"
          onClick={() =>
            window.scrollBy({ top: -window.innerHeight, behavior: 'smooth' })
          }
        >
          <svg viewBox="0 0 24 24" width="28" height="28" aria-hidden>
            <path
              d="M6 15l6-6 6 6"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>
      )}
    </div>
  )
}
