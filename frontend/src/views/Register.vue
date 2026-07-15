<template>
  <div class="auth-wrapper">
    <div class="auth-card">
      <div class="logo-container center">
        <div class="logo-icon">
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

      <!-- Étape 1 : Formulaire d'inscription -->
      <div v-if="!verificationStep">
        <h1 class="auth-title">Créer un compte</h1>
        <p class="auth-subtitle">
          Déjà inscrit ? 
          <router-link to="/login" class="link-highlight">Se connecter</router-link>
        </p>

        <div v-if="errorMsg" class="alert alert-danger">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
          <span>{{ errorMsg }}</span>
        </div>

        <form @submit.prevent="handleRegister">
          <div class="form-row">
            <div class="form-group">
              <label for="nom">Nom</label>
              <input type="text" id="nom" v-model="form.nom" placeholder="Martin" required />
            </div>
            <div class="form-group">
              <label for="prenom">Prénom</label>
              <input type="text" id="prenom" v-model="form.prenom" placeholder="Lucas" required />
            </div>
          </div>

          <div class="form-group">
            <label for="email">Adresse e-mail</label>
            <input type="email" id="email" v-model="form.email" placeholder="lucas.martin@mail.fr" required />
          </div>

          <div class="form-group">
            <label for="adresse">Adresse complète</label>
            <input type="text" id="adresse" v-model="form.adresse" placeholder="12 rue de Paris, 75001 Paris" required />
          </div>

          <div class="form-row">
            <div class="form-group">
              <label for="telephone">Téléphone</label>
              <input type="tel" id="telephone" v-model="form.telephone" placeholder="0601020304" />
            </div>
            <div class="form-group">
              <label for="dob">Date de naissance</label>
              <input type="date" id="dob" v-model="form.date_naissance" required @change="checkAge" />
            </div>
          </div>

          <div class="form-group">
            <label for="salle">Salle principale</label>
            <select id="salle" v-model="form.salle_id" required>
              <option value="" disabled>Sélectionnez une salle</option>
              <option v-for="s in salles" :key="s.id" :value="s.id">{{ s.nom }}</option>
            </select>
          </div>

          <!-- Avertissement si moins de 16 ans -->
          <div v-if="age !== null && age < 16" class="alert alert-danger">
            L'inscription est interdite aux mineurs de moins de 16 ans.
          </div>

          <!-- Case à cocher accord parental si mineur (16-17 ans) -->
          <div v-if="age !== null && age >= 16 && age < 18" class="form-group checkbox-group">
            <label class="checkbox-label">
              <input type="checkbox" v-model="form.parental_consent" required />
              Je certifie avoir l'accord écrit de mes représentants légaux pour m'inscrire.
            </label>
          </div>

          <div class="form-group">
            <label for="password">Mot de passe</label>
            <input type="password" id="password" v-model="form.password" placeholder="••••••••" required />
          </div>

          <button 
            type="submit" 
            class="btn btn-primary adherent" 
            :disabled="loading || (age !== null && age < 16)"
          >
            <span v-if="loading" class="spinner-small"></span>
            <span v-else>S'inscrire</span>
          </button>
        </form>
      </div>

      <!-- Étape 2 : Simulation validation email (Démo) -->
      <div v-else class="success-step">
        <div class="success-icon">
          <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect width="20" height="16" x="2" y="4" rx="2"/><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/>
          </svg>
        </div>
        <h1 class="auth-title">Vérifiez vos e-mails</h1>
        <p class="auth-subtitle">
          Un e-mail de validation a été envoyé à <strong>{{ form.email }}</strong> pour activer votre compte.
        </p>

        <div v-if="verificationSuccess" class="alert alert-success">
          Compte activé avec succès ! Vous pouvez maintenant vous connecter.
        </div>
        <div v-if="verificationError" class="alert alert-danger">
          {{ verificationError }}
        </div>

        <div class="demo-box-verify">
          <span class="demo-tag">MODE DÉMO UML</span>
          <p>Le backend a généré le token de validation. Vous pouvez l'activer immédiatement ci-dessous sans consulter vos e-mails.</p>
          
          <button 
            type="button" 
            class="btn btn-primary adherent" 
            @click="simulateVerification"
            :disabled="verifying || verificationSuccess"
          >
            <span v-if="verifying" class="spinner-small"></span>
            <span v-else-if="verificationSuccess">Compte Activé !</span>
            <span v-else>Activer mon compte automatiquement</span>
          </button>
        </div>

        <router-link to="/login" class="btn btn-secondary mt-16">
          Retourner à la page de connexion
        </router-link>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, reactive, onMounted } from 'vue';

export default {
  name: 'Register',
  setup() {
    const form = reactive({
      nom: '',
      prenom: '',
      email: '',
      telephone: '',
      date_naissance: '',
      adresse: '',
      salle_id: '',
      password: '',
      parental_consent: false,
    });

    const age = ref(null);
    const loading = ref(false);
    const errorMsg = ref('');
    const salles = ref([]);
    
    // Étape de vérification
    const verificationStep = ref(false);
    const generatedToken = ref('');
    const verifying = ref(false);
    const verificationSuccess = ref(false);
    const verificationError = ref('');

    // Charger les salles depuis le backend
    const fetchSalles = async () => {
      try {
        const res = await fetch('http://localhost:8080/salles');
        const data = await res.json();
        if (res.ok && data.salles) {
          salles.value = data.salles;
          if (data.salles.length > 0) {
            form.salle_id = data.salles[0].id;
          }
        }
      } catch (err) {
        console.error("Erreur lors de la récupération des salles :", err);
      }
    };

    onMounted(fetchSalles);

    // Calculer l'âge
    const checkAge = () => {
      if (!form.date_naissance) {
        age.value = null;
        return;
      }
      const birthDate = new Date(form.date_naissance);
      const today = new Date();
      let calculatedAge = today.getFullYear() - birthDate.getFullYear();
      const monthDiff = today.getMonth() - birthDate.getMonth();
      if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < birthDate.getDate())) {
        calculatedAge--;
      }
      age.value = calculatedAge;
    };

    const handleRegister = async () => {
      loading.value = true;
      errorMsg.value = '';

      if (age.value < 16) {
        errorMsg.value = "L'âge minimum requis est de 16 ans.";
        loading.value = false;
        return;
      }

      try {
        const payload = {
          nom: form.nom,
          prenom: form.prenom,
          email: form.email,
          telephone: form.telephone,
          date_naissance: form.date_naissance,
          adresse: form.adresse,
          salle_id: parseInt(form.salle_id),
          mot_de_passe: form.password,
          accord_parental: form.parental_consent,
        };

        const res = await fetch('http://localhost:8080/auth/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        });

        const data = await res.json();
        if (!res.ok) {
          throw new Error(data.error || "Erreur lors de l'inscription");
        }

        generatedToken.value = data.token; // Jeton renvoyé par le backend
        verificationStep.value = true;
      } catch (err) {
        errorMsg.value = err.message || "Erreur réseau";
      } finally {
        loading.value = false;
      }
    };

    // Simuler le clic sur le lien reçu par e-mail
    const simulateVerification = async () => {
      verifying.value = true;
      verificationError.value = '';
      try {
        const res = await fetch(`http://localhost:8080/auth/verify/${generatedToken.value}`);
        const data = await res.json();
        
        if (!res.ok) {
          throw new Error(data.error || "Le jeton d'activation est invalide");
        }
        
        verificationSuccess.value = true;
      } catch (err) {
        verificationError.value = err.message;
      } finally {
        verifying.value = false;
      }
    };

    return {
      form,
      age,
      loading,
      errorMsg,
      salles,
      verificationStep,
      generatedToken,
      verifying,
      verificationSuccess,
      verificationError,
      checkAge,
      handleRegister,
      simulateVerification,
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
  max-width: 500px;
  padding: 40px;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3);
}

.logo-container.center {
  justify-content: center;
  margin-bottom: 24px;
}

.auth-title {
  font-size: 26px;
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

.checkbox-group {
  display: flex;
  align-items: flex-start;
  padding: 12px;
  background-color: rgba(249, 115, 22, 0.05);
  border: 1px solid rgba(249, 115, 22, 0.1);
  border-radius: 10px;
}

.checkbox-label {
  display: flex;
  gap: 10px;
  font-size: 13px;
  line-height: 1.4;
  color: var(--text-primary);
  cursor: pointer;
}

.checkbox-label input {
  width: auto;
  margin-top: 3px;
  cursor: pointer;
}

.success-step {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.success-icon {
  width: 64px;
  height: 64px;
  background-color: rgba(249, 115, 22, 0.1);
  color: var(--accent-adherent);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 20px;
  border: 1px solid rgba(249, 115, 22, 0.2);
}

.demo-box-verify {
  width: 100%;
  margin-top: 24px;
  background-color: rgba(255,255,255,0.02);
  border: 1px dashed var(--border-color);
  border-radius: 12px;
  padding: 20px;
}

.demo-tag {
  display: inline-block;
  font-size: 10px;
  font-weight: 700;
  background-color: rgba(255,255,255,0.08);
  padding: 2px 6px;
  border-radius: 4px;
  margin-bottom: 8px;
  letter-spacing: 0.5px;
}

.demo-box-verify p {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.5;
  margin-bottom: 16px;
}

.mt-16 {
  margin-top: 16px;
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
