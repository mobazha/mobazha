import Reactotron from 'reactotron-react-native';
import { reactotronRedux } from 'reactotron-redux';

if(__DEV__) {
  Reactotron
    .configure({ host: 'localhost' }) // controls connection & communication settings
    .use(reactotronRedux())
    .useReactNative() // add all built-in react native plugins
    .connect(); // let's connect!
}
