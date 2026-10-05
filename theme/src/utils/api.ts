/**
 * Komari API 客户端 SDK
 * 基于 REST API 的 Komari 客户端
 * @see https://www.komari.wiki/dev/api.html
 */

// ==================== 类型定义 ====================

/** API 响应基础结构 */
interface ApiResponse<T = unknown> {
  status: 'success' | 'error'
  message: string
  data: T
}

/** 用户信息 */
export interface MeInfo {
  'logged_in': boolean
  'username': string
  '2fa_enabled'?: boolean
  'sso_id'?: string
  'sso_type'?: string
  'uuid'?: string
}

/** 公开站点属性 */
export interface PublicSettings {
  allow_cors: boolean
  custom_body: string
  custom_head: string
  description: string
  disable_password_login: boolean
  oauth_enable: boolean
  oauth_provider: string | null
  ping_record_preserve_time: number
  record_enabled: boolean
  record_preserve_time: number
  sitename: string
  theme: string
  theme_settings?: Record<string, unknown> | null
  /** 数据更新间隔（秒），主题配置项 */
  dataUpdateInterval?: number
}

/** 版本信息 */
export interface VersionInfo {
  hash: string
  version: string
}
/** Ping 历史记录 */
export interface PingRecord {
  task_id: number
  time: string
  value: number
}

/** Ping 任务信息 */
export interface PingTask {
  id: number
  interval: number
  name: string
  loss: number
}
/** API 客户端配置 */
export interface ApiClientOptions {
  /** 基础路径，默认 '/api' */
  baseUrl?: string
  /** 超时时间（毫秒），默认 30000 */
  timeout?: number
}

/** API 错误 */
class ApiError extends Error {
  status: string
  code?: number

  constructor(message: string, status: string = 'error', code?: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

// ==================== API 客户端 ====================

/** Komari API 客户端 */
class KomariApi {
  private baseUrl: string
  private timeout: number

  constructor(options: ApiClientOptions = {}) {
    this.baseUrl = options.baseUrl || import.meta.env.VITE_API_BASE || '/api'
    this.timeout = options.timeout || 30000
  }

  /**
   * 发送 GET 请求
   */
  private async get<T>(path: string, params?: Record<string, string | number>): Promise<T> {
    let url = `${this.baseUrl}${path}`
    if (params) {
      const searchParams = new URLSearchParams()
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== null) {
          searchParams.append(key, String(value))
        }
      })
      const queryString = searchParams.toString()
      if (queryString) {
        url += `?${queryString}`
      }
    }

    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const response = await fetch(url, {
        method: 'GET',
        credentials: 'include', // 携带 Cookie
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      const result: ApiResponse<T> = await response.json()

      if (result.status === 'error') {
        throw new ApiError(result.message || 'Unknown error', 'error', response.status)
      }

      return result.data
    }
    catch (error) {
      clearTimeout(timeoutId)
      if (error instanceof ApiError)
        throw error
      throw new ApiError(`Network error: ${error instanceof Error ? error.message : String(error)}`, 'error')
    }
  }

  /**
   * 发送 GET 请求（直接返回响应，不解析 ApiResponse 结构）
   */
  private async getRaw<T>(path: string): Promise<T> {
    const url = `${this.baseUrl}${path}`

    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const response = await fetch(url, {
        method: 'GET',
        credentials: 'include',
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      if (!response.ok) {
        throw new ApiError(`HTTP error: ${response.status}`, 'error', response.status)
      }

      return await response.json()
    }
    catch (error) {
      clearTimeout(timeoutId)
      if (error instanceof ApiError)
        throw error
      throw new ApiError(`Network error: ${error instanceof Error ? error.message : String(error)}`, 'error')
    }
  }

  /**
   * 发送 POST 请求
   */
  private async post<T>(path: string, body?: unknown): Promise<T> {
    const url = `${this.baseUrl}${path}`

    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: body ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      const result = await response.json()

      // 登录接口返回 set-cookie 特殊结构
      if (result['set-cookie']) {
        return result as T
      }

      // 检查 API 响应状态
      const apiResult: ApiResponse<T> = result
      if (apiResult.status === 'error') {
        throw new ApiError(apiResult.message || 'Unknown error', 'error', response.status)
      }

      return apiResult.data
    }
    catch (error) {
      clearTimeout(timeoutId)
      if (error instanceof ApiError)
        throw error
      throw new ApiError(`Network error: ${error instanceof Error ? error.message : String(error)}`, 'error')
    }
  }

  // ===== 用户信息接口 =====

  /**
   * 获取当前用户信息
   * 注意：此接口返回的是直接的 MeInfo 对象，不是包裹在 { status, message, data } 中
   */
  async getMe(): Promise<MeInfo> {
    return this.getRaw<MeInfo>('/me')
  }

  // ===== 服务端公开属性 =====

  /**
   * 获取站点的公开设置属性
   */
  async getPublicSettings(): Promise<PublicSettings> {
    return this.get<PublicSettings>('/public')
  }

  /**
   * 获取服务端版本信息
   */
  async getVersion(): Promise<VersionInfo> {
    return this.get<VersionInfo>('/version')
  }
}

// ==================== 单例实例 ====================

let sharedApiInstance: KomariApi | null = null

/**
 * 获取共享的 KomariApi 实例
 */
export function getSharedApi(options?: ApiClientOptions): KomariApi {
  if (!sharedApiInstance) {
    sharedApiInstance = new KomariApi(options)
  }
  return sharedApiInstance
}

// 默认导出
export default KomariApi
