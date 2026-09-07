import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import test from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const src = readFileSync(join(here, 'Login.vue'), 'utf8')

test('login page hides create-account CTA unless self-serve or invite token', () => {
  assert.match(src, /registrationEnabled \|\| inviteLookup/)
  // Plain login (no invite): CTA still gated — inviteLookup alone is not enough without the OR of registrationEnabled in the same expression as used for the CTA.
  assert.match(
    src,
    /class="register-cta" v-if="registrationEnabled \|\| inviteLookup"/,
  )
})
