import { Nav, Navbar, NavDropdown, Container } from "react-bootstrap";
import { NavLink } from "react-router-dom";
import {
  BsGrid,
  BsFiles,
  BsPlug,
  BsTablet,
  BsDisplay,
  BsActivity,
  BsPeople,
  BsEnvelope,
} from "react-icons/bs";
import { logout } from "../common/actions";
import { useAuthState } from "../common/useAuthContext";
import useTheme from "../common/useTheme";

export default function NavigationBar() {
  const {
    state: { user },
    dispatch,
  } = useAuthState();
  const [theme, setTheme] = useTheme();
  const links = [
    ["/", "Overview", BsGrid],
    ["/documents", "Documents", BsFiles],
    ["/integrations", "Integrations", BsPlug],
    ["/connect", "Connect", BsTablet],
    ["/screenshare", "Screen share", BsDisplay],
  ];
  if (user?.Roles?.includes("Admin"))
    links.push(
      ["/health", "Health", BsActivity],
      ["/settings/mail", "Mail", BsEnvelope],
      ["/admin", "Users", BsPeople],
    );
  return (
    <Navbar expand="xl" className="app-navigation" collapseOnSelect>
      <Container fluid>
        <Navbar.Brand as={NavLink} to="/" className="wordmark">
          <span className="brand-icon">
            <img src="/assets/brand/icon.png" alt="" width="36" height="36" />
          </span>
          <span className="brand-name">
            rmfake<span>cloud</span>
          </span>
        </Navbar.Brand>
        <div className="nav-controls">
          <label className="theme-control">
            <span className="visually-hidden">Color theme</span>
            <select
              aria-label="Color theme"
              value={theme}
              onChange={(e) => setTheme(e.target.value)}
            >
              <option value="system">System theme</option>
              <option value="light">Light theme</option>
              <option value="dark">Dark theme</option>
            </select>
          </label>
          <Navbar.Toggle aria-controls="main-navigation" />
        </div>
        {user && (
          <Navbar.Collapse id="main-navigation">
            <Nav className="primary-navigation">
              {links.map(([path, title, Icon]) => (
                <Nav.Link
                  key={path}
                  eventKey={path}
                  as={NavLink}
                  exact={path === "/"}
                  to={path}
                >
                  <Icon />
                  <span>{title}</span>
                </Nav.Link>
              ))}
            </Nav>
            <Nav className="ms-auto">
              <NavDropdown id="user-menu" title={user.UserID} align="end">
                <NavDropdown.Item as={NavLink} to="/profile">
                  Profile
                </NavDropdown.Item>
                <NavDropdown.Item onClick={() => logout(dispatch)}>
                  Log out
                </NavDropdown.Item>
              </NavDropdown>
            </Nav>
          </Navbar.Collapse>
        )}
      </Container>
    </Navbar>
  );
}
