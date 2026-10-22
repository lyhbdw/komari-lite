interface Job {
  run: () => Promise<unknown>
  resolve: (value: unknown) => void
  reject: (reason: unknown) => void
  signal?: AbortSignal
  abort: () => void
}
const queue: Job[] = []
let running = false
const MAX_QUEUED = 8

function aborted(): DOMException {
  return new DOMException('History request cancelled', 'AbortError')
}
function pause(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(aborted())
      return
    }
    let timer: ReturnType<typeof setTimeout>
    const abort = () => {
      clearTimeout(timer)
      reject(aborted())
    }
    timer = setTimeout(() => {
      signal?.removeEventListener('abort', abort)
      resolve()
    }, ms)
    signal?.addEventListener('abort', abort, { once: true })
  })
}
async function retry(run: () => Promise<unknown>, signal?: AbortSignal): Promise<unknown> {
  for (let attempt = 0; ; attempt++) {
    if (signal?.aborted)
      throw aborted()
    try {
      return await run()
    }
    catch (error) {
      const e = error as { code?: number, message?: string }
      if (attempt >= 2 || e?.code !== -32051 || e.message !== 'too many concurrent history queries')
        throw error
      await pause(150 * 2 ** attempt, signal)
    }
  }
}
async function drain(): Promise<void> {
  if (running)
    return
  const job = queue.shift()
  if (!job)
    return
  running = true
  job.signal?.removeEventListener('abort', job.abort)
  try {
    job.resolve(await retry(job.run, job.signal))
  }
  catch (error) {
    job.reject(error)
  }
  finally {
    running = false
    void drain()
  }
}
// A single shared browser slot leaves room for other server callers. The
// bounded queue rejects overload and removes cancelled work immediately.
export function scheduleHistory<T>(run: () => Promise<T>, signal?: AbortSignal): Promise<T> {
  if (signal?.aborted)
    return Promise.reject(aborted())
  if (queue.length >= MAX_QUEUED)
    return Promise.reject(new Error('History request queue is full'))
  return new Promise<T>((resolve, reject) => {
    const job: Job = {
      run,
      resolve: value => resolve(value as T),
      reject,
      signal,
      abort: () => {
        const index = queue.indexOf(job)
        if (index >= 0)
          queue.splice(index, 1)
        reject(aborted())
      },
    }
    signal?.addEventListener('abort', job.abort, { once: true })
    queue.push(job)
    void drain()
  })
}
