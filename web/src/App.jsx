import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import './App.css'

// Approximate geographic center of China; used when locating fails.
const NATIONWIDE_LOC = { lat: 35.8617, lng: 104.1954 }
// Large enough to cover China from any domestic coordinate.
const NATIONWIDE_RADIUS_KM = 6000

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
    radius_km: String(radiusKm ?? NATIONWIDE_RADIUS_KM),
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

async function locateByIP() {
  const res = await fetch('/api/v1/geo/ip')
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

function geoErrorHint(err) {
  if (!err || typeof err.code !== 'number') return '定位失败'
  if (err.code === 0) return '浏览器不支持定位'
  if (err.code === 1) return '已拒绝定位权限'
  if (err.code === 2) return '无法获取位置'
  if (err.code === 3) return '定位超时'
  return '定位失败'
}

function browserLocate(options) {
  return new Promise((resolve, reject) => {
    if (!navigator.geolocation) {
      reject(Object.assign(new Error('unsupported'), { code: 0 }))
      return
    }
    navigator.geolocation.getCurrentPosition(resolve, reject, options)
  })
}

export default function App() {
  const [loc, setLoc] = useState(null)
  const [placeName, setPlaceName] = useState('定位中')
  const [locHint, setLocHint] = useState('正在获取当前位置…')
  const [locating, setLocating] = useState(false)
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
  const locateSeq = useRef(0)

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

  const applyLocation = useCallback(async (next, hint, seq) => {
    if (seq !== locateSeq.current) return
    setLoc(next)
    try {
      const place = await reversePlace(next.lat, next.lng)
      if (seq !== locateSeq.current) return
      setPlaceName(place.name || place.city || '未知')
      setLocHint(hint || `${next.lat.toFixed(3)}, ${next.lng.toFixed(3)}`)
    } catch {
      if (seq !== locateSeq.current) return
      setPlaceName(`${next.lat.toFixed(2)}, ${next.lng.toFixed(2)}`)
      setLocHint(hint || '坐标')
    }
  }, [])

  const useNationwide = useCallback((hint, seq) => {
    if (seq !== locateSeq.current) return
    setLoc(NATIONWIDE_LOC)
    setPlaceName('全国')
    setLocHint(hint || '搜索全国范围')
  }, [])

  const locateMe = useCallback(async () => {
    const seq = ++locateSeq.current
    setLocating(true)
    setPlaceName('定位中')
    setLocHint('正在获取当前位置…')

    try {
      if (!navigator.geolocation) {
        useNationwide('浏览器不支持定位，已切换全国', seq)
        return
      }

      try {
        const pos = await browserLocate({
          enableHighAccuracy: false,
          timeout: 12000,
          maximumAge: 60000,
        })
        if (seq !== locateSeq.current) return
        await applyLocation(
          { lat: pos.coords.latitude, lng: pos.coords.longitude },
          '当前位置',
          seq,
        )
        return
      } catch (browserErr) {
        if (seq !== locateSeq.current) return
        setLocHint(`${geoErrorHint(browserErr)}，尝试网络定位…`)
      }

      try {
        const place = await locateByIP()
        if (seq !== locateSeq.current) return
        const next = { lat: place.lat, lng: place.lng }
        setLoc(next)
        setPlaceName(place.name || place.city || '未知')
        setLocHint('网络定位（公网 IP 近似）')
        return
      } catch {
        if (seq !== locateSeq.current) return
      }

      useNationwide('定位失败，已切换全国', seq)
    } finally {
      if (seq === locateSeq.current) setLocating(false)
    }
  }, [applyLocation, useNationwide])

  const runDiscover = useCallback(async (overrides = {}) => {
    const center = overrides.lat != null
      ? { lat: overrides.lat, lng: overrides.lng }
      : loc
    if (!center) return
    setStatus('正在发现附近 Agent…')
    const q = overrides.q ?? query
    const cat = overrides.category ?? category
    const tags = overrides.tags ?? tag
    try {
      const list = await discoverAgents({
        lat: center.lat,
        lng: center.lng,
        q,
        category: cat,
        tags,
        radiusKm: overrides.radiusKm ?? NATIONWIDE_RADIUS_KM,
      })
      setAgents(list)
      setStatus(list.length ? `找到 ${list.length} 个 Agent` : '未找到匹配的 Agent')
    } catch (e) {
      setStatus(`发现失败：${e.message}`)
    }
  }, [loc, query, category, tag])

  useEffect(() => {
    locateMe()
  }, [locateMe])

  useEffect(() => {
    if (!loc) return
    runDiscover()
  }, [loc?.lat, loc?.lng])

  useEffect(() => {
    if (!loc || !query) return
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
    const center = loc || NATIONWIDE_LOC
    setBusy(true)
    setHostMessages((m) => [...m, { role: 'user', text: message }])
    setHostInput('')
    try {
      const res = await fetch('/api/v1/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message, lat: center.lat, lng: center.lng }),
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
        <button
          className="loc-pill"
          type="button"
          title={`${locHint}（点击重新定位）`}
          onClick={locateMe}
          disabled={locating}
        >
          <span className="loc-place">{placeName}</span>
          <span className="loc-hint">{locating ? '定位中…' : locHint}</span>
        </button>
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
          {speech.supported && (
            <button
              className={`btn btn-voice ${speech.listening ? 'listening' : ''}`}
              type="button"
              onClick={speech.toggle}
            >
              {speech.listening ? '聆听中…' : '语音'}
            </button>
          )}
          <button className="btn btn-primary" type="button" onClick={() => runDiscover()}>
            搜索
          </button>
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
          <button className="btn btn-ghost" type="button" onClick={locateMe} disabled={locating}>
            {locating ? '定位中…' : '重新定位'}
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
