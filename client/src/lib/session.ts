import { get, writable } from "svelte/store"
import { idToken } from "./auth/auth"

const apiBaseUrl = "http://localhost:3333"

export const activeSessison = writable(false)

export const startSession = () => {
	activeSessison.set(true)
}

export const stopSession = () => {
	activeSessison.set(false)
}
