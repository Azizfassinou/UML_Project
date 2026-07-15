<template>
  <div class="auth-wrapper">
    <div class="auth-card">
      <div class="logo-container center">
        <div class="logo-icon">
          <!-- Dumbbell SVG Icon -->
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="m6.5 6.5 11 11"/>
            <path d="m21 21-1-1"/>
            <path d="m3 3 1 1"/>
            <path d="m18 22 4-4"/>
            <path d="m2 6 4-4"/>
            <path d="m3 10 7-7"/>
            <path d="m14 21 7-7"/>
            <path d="M6.5 12.5 12.5 6.5"/>
            <path d="m11.5 17.5 6-6"/>
          </svg>
        </div>
        <span class="logo-text">FAV<span class="logo-accent">FIT</span></span>
      </div>

      <h1 class="auth-title">Connexion</h1>
      <p class="auth-subtitle">
        Pas encore de compte ? 
        <router-link to="/register" class="link-highlight">S'inscrire</router-link>
      </p>

      <div class="demo-selector-box">
        <span class="demo-label">DÉMO — CHOISIR UN RÔLE</span>
        <div class="demo-buttons">
          <button 
            type="button" 
            class="demo-btn" 
            :class="{ active: selectedRole === 'adherent' }"
            @click="selectRole('adherent')"
          >
            Adhérent
          </button>
          <button 
            type="button" 
            class="demo-btn" 
            :class="{ active: selectedRole === 'gestionnaire' }"
            @click="selectRole('gestionnaire')"
          >
            Admin
          </button>
          <button 
            type="button" 
            class="demo-btn" 
            :class="{ active: selectedRole === 'coach' }"
            @click="selectRole('coach')"
          >
            Coach
          </button>
        </div>
      </div>

      <div v-if="errorMsg" class="alert alert-danger">
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
        <span>{{ errorMsg }}</span>
      </div>

      <form @submit.prevent="handleLogin">
        <div class="form-group">
          <label for="email">Adresse e-mail</label>
          <input 
            type="email" 
            id="email" 
            v-model="email" 
            placeholder="vous@exemple.fr" 
            required 
          />
        </div>

        <div class="form-group">
          <div class="label-row">
            <label for="password">Mot de passe</label>
            <a href="#" class="link-forgot" @click.prevent="showForgotAlert">Mot de passe oublié ?</a>
          </div>
          <div class="password-input-wrapper">
            <input 
              :type="showPassword ? 'text' : 'password'" 
              id="password" 
              v-model="password" 
              placeholder="••••••••" 
              required 
            />
            <button 
              type="button" 
              class="password-toggle" 
              @click="showPassword = !showPassword"
            >
              <!-- Eye / EyeOff SVG -->
              <svg v-if="showPassword" xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M9.88 9.88a3 3 0 1 0 4.24 4.24"/>
                <path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68"/>
                <path d="M6.61 6.61A13.52 13.52 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61"/>
                <line x1="2" y1="2" x2="22" y2="22"/>
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/>
                <circle cx="12" cy="12" r="3"/>
              </svg>
            </button>
          </div>
        </div>

        <button 
          type="submit" 
          class="btn btn-primary" 
          :class="selectedRole" 
          :disabled="loading"
        >
          <span v-if="loading" class="spinner-small"></span>
          <span v-else>Se connecter en tant que {{ roleLabel }}</span>
        </button>
      </form>
    </div>
  </div>
</template>

<script>
import { ref, computed } from 'vue';
import { useRouter } from 'vue-router';
import { login, getRoleLabel } from '../auth.js';

export default {
  name: 'Login',
  setup() {
    const router = useRouter();
    const selectedRole = ref('adherent');
    const email = ref('lucas.martin@mail.fr');
    const password = ref('password123');
    const showPassword = ref(false);
    const loading = ref(false);
    const errorMsg = ref('');

    const roleLabel = computed(() => {
      if (selectedRole.value === 'adherent') return 'Adhérent';
      if (selectedRole.value === 'gestionnaire') return 'Administrateur';
      if (selectedRole.value === 'coach') return 'Coach';
      return '';
    });

    const selectRole = (role) => {
      selectedRole.value = role;
      errorMsg.value = '';
      
      // Auto-fill les informations de démo
      if (role === 'adherent') {
        email.value = 'lucas.martin@mail.fr';
      } else if (role === 'gestionnaire') {
        email.value = 'jean.gestion@fitflow.fr';
      } else if (role === 'coach') {
        email.value = 'sofia.diallo@fitflow.fr';
      }
      password.value = 'password123';
    };

    const handleLogin = async () => {
      loading.value = true;
      errorMsg.value = '';
      try {
        await login(email.value, password.value, selectedRole.value);
        router.push({ name: 'Dashboard' });
      } catch (err) {
        errorMsg.value = err.message || 'Identifiants incorrects';
      } finally {
        loading.value = false;
      }
    };

    const showForgotAlert = () => {
      alert("Pour la démo, utilisez les identifiants préremplis (mot de passe : 'password123').");
    };

    return {
      selectedRole,
      email,
      password,
      showPassword,
      loading,
      errorMsg,
      roleLabel,
      selectRole,
      handleLogin,
      showForgotAlert,
    };
  },
};
</script>

<style scoped>
.auth-wrapper {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--bg-primary);
  padding: 24px;
}

.auth-card {
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-color);
  border-radius: 20px;
  width: 100%;
  max-width: 460px;
  padding: 40px;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3);
}

.logo-container.center {
  justify-content: center;
  margin-bottom: 24px;
}

.auth-title {
  font-size: 28px;
  font-weight: 700;
  text-align: center;
  margin-bottom: 8px;
  color: white;
}

.auth-subtitle {
  text-align: center;
  color: var(--text-secondary);
  font-size: 14px;
  margin-bottom: 32px;
}

.link-highlight {
  color: var(--accent-adherent);
  font-weight: 600;
}

.link-highlight:hover {
  text-decoration: underline;
}

.demo-selector-box {
  background-color: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 24px;
}

.demo-label {
  display: block;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: 12px;
  letter-spacing: 0.5px;
}

.demo-buttons {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 8px;
}

.demo-btn {
  background-color: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--border-color);
  padding: 8px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.demo-btn:hover {
  background-color: rgba(255, 255, 255, 0.06);
}

.demo-btn.active {
  color: white;
  border-color: transparent;
}

/* Rôles couleurs d'actifs */
.demo-selector-box:has(.demo-btn.active:nth-child(1)) .demo-btn.active {
  background-color: var(--accent-adherent);
}

.demo-selector-box:has(.demo-btn.active:nth-child(2)) .demo-btn.active {
  background-color: var(--accent-admin);
}

.demo-selector-box:has(.demo-btn.active:nth-child(3)) .demo-btn.active {
  background-color: var(--accent-coach);
}

.label-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.link-forgot {
  font-size: 12px;
  color: var(--accent-adherent);
  font-weight: 500;
}

.link-forgot:hover {
  text-decoration: underline;
}

.password-input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.password-toggle {
  position: absolute;
  right: 16px;
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4px;
}

.password-toggle:hover {
  color: white;
}

.spinner-small {
  width: 20px;
  height: 20px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
</style>
