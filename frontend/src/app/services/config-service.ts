import { types } from '../../../wailsjs/go/models';
import { apiService } from './api';

class ConfigService {
  async getConfig() {
    return apiService.getConfig();
  }

  async updateConfig(config: types.AppConfig) {
    return apiService.updateConfig(config);
  }

  onConfigUpdate(callback: (config: types.AppConfig) => void) {
    return apiService.onConfigUpdate(callback);
  }
}

export const configService = new ConfigService();
