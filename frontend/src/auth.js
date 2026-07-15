// auth.js : Gestion de l'authentification et requêtes API.
import { reactive, watch } from 'vue';

const API_BASE_URL = 'http://localhost:8080';

export const authState = reactive({
  accessToken: localStorage.getItem('access_token') || null,
  refreshToken: localStorage.getItem('refresh_token') || null,
  user: JSON.parse(localStorage.getItem('user_info')) || null, // { id, nom, prenom, email, role }
  isAuthenticated: !!localStorage.getItem('access_token'),
});

// Surveiller l'état pour synchroniser avec localStorage
watch(
  () => authState.accessToken,
  (val) => {
    if (val) localStorage.setItem('access_token', val);
    else localStorage.removeItem('access_token');
    authState.isAuthenticated = !!val;
  }
);

watch(
  () => authState.refreshToken,
  (val) => {
    if (val) localStorage.setItem('refresh_token', val);
    else localStorage.removeItem('refresh_token');
  }
);

watch(
  () => authState.user,
  (val) => {
    if (val) localStorage.setItem('user_info', JSON.stringify(val));
    else localStorage.removeItem('user_info');
  },
  { deep: true }
);

// Obtenir la couleur d'accentuation CSS en fonction du rôle
export function getRoleAccentColor(role) {
  switch (role) {
    case 'gestionnaire':
      return '#a855f7'; // Violet
    case 'coach':
      return '#22c55e'; // Vert
    case 'adherent':
    default:
      return '#f97316'; // Orange
  }
}

// Obtenir le libellé français du rôle
export function getRoleLabel(role) {
  switch (role) {
    case 'gestionnaire':
      return 'Back-office Admin';
    case 'coach':
      return 'Espace Coach';
    case 'adherent':
    default:
      return 'Espace Adhérent';
  }
}

// Connexion de l'utilisateur
export async function login(email, password, expectedRole) {
  try {
    const res = await fetch(`${API_BASE_URL}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, mot_de_passe: password }),
    });

    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || 'Erreur lors de la connexion');
    }

    // Valider le rôle
    if (data.user.role !== expectedRole) {
      throw new Error(`Accès refusé : Ce compte n'a pas le rôle ${getRoleLabel(expectedRole)}`);
    }

    authState.accessToken = data.access_token;
    authState.refreshToken = data.refresh_token;
    authState.user = data.user;

    return data.user;
  } catch (err) {
    throw err;
  }
}

// Déconnexion
export function logout() {
  authState.accessToken = null;
  authState.refreshToken = null;
  authState.user = null;
}

// Rafraîchir l'Access Token avec le Refresh Token
export async function refreshAccessToken() {
  if (!authState.refreshToken) {
    logout();
    return null;
  }

  try {
    const res = await fetch(`${API_BASE_URL}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: authState.refreshToken }),
    });

    const data = await res.json();
    if (!res.ok) {
      throw new Error('Session expirée');
    }

    authState.accessToken = data.access_token;
    authState.refreshToken = data.refresh_token;
    return data.access_token;
  } catch (err) {
    logout();
    return null;
  }
}

// Wrapper fetch pour gérer automatiquement l'authentification et le rafraîchissement des jetons
export async function apiFetch(path, options = {}) {
  const url = `${API_BASE_URL}${path}`;

  // Cloner et préparer les headers
  const headers = { ...options.headers };
  if (authState.accessToken) {
    headers['Authorization'] = `Bearer ${authState.accessToken}`;
  }

  if (options.body && !(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }

  const fetchOptions = {
    ...options,
    headers,
  };

  let response = await fetch(url, fetchOptions);

  // Si jeton expiré (401), tenter le rafraîchissement
  if (response.status === 401 && authState.refreshToken) {
    const newAccessToken = await refreshAccessToken();
    if (newAccessToken) {
      // Re-tenter la requête d'origine avec le nouveau jeton
      headers['Authorization'] = `Bearer ${newAccessToken}`;
      response = await fetch(url, fetchOptions);
    }
  }

  return response;
}
