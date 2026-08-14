/** Client-side mirror of backend auth.ValidatePassword (min 10, letter+digit, common list). */
export const MIN_PASSWORD_LEN = 10;

const COMMON = new Set([
  'admin',
  'operator',
  'password',
  'password1',
  'password12',
  'password123',
  'changeme',
  '123456',
  '12345678',
  '1234567890',
  'qwerty',
  'qwerty123',
  'letmein',
  'welcome',
  'welcome1',
  'admin123',
  'administrator',
  'passw0rd',
  'p@ssw0rd',
  'secret',
  'default',
]);

export function validatePasswordClient(password: string, username?: string): string | null {
  if (password.length < MIN_PASSWORD_LEN) {
    return `Минимум ${MIN_PASSWORD_LEN} символов, буква и цифра`;
  }
  if (password.length > 128) {
    return 'Пароль слишком длинный';
  }
  const lower = password.toLowerCase().trim();
  if (username && lower === username.toLowerCase().trim()) {
    return 'Пароль не должен совпадать с логином';
  }
  if (COMMON.has(lower)) {
    return 'Слишком простой пароль';
  }
  const hasLetter = /[A-Za-zА-Яа-яЁё]/.test(password);
  const hasDigit = /\d/.test(password);
  if (!hasLetter || !hasDigit) {
    return 'Нужны буква и цифра';
  }
  return null;
}
